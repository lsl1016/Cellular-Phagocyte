package game

import (
	"sort"

	"cellular-phagocyte/server/internal/protocol"
)

// aoiView 是服务端为单个玩家计算出的圆形兴趣区域。
type aoiView struct {
	centerX float64
	centerY float64
	radius  float64
}

// roomAOIIndex 是一次快照周期内的只读空间索引。
// 它复用碰撞热路径已经落地的 spatialGrid 实现，但不会改变权威房间状态。
type roomAOIIndex struct {
	balls             *spatialGrid[ballRef]
	foods             *spatialGrid[*Food]
	ejected           *spatialGrid[*EjectedMass]
	maxBallRadius     float64
	maxEjectedRadius  float64
	foodRadius        float64
}

func (r *Room) buildAOIIndexLocked() *roomAOIIndex {
	idx := &roomAOIIndex{
		balls:   newSpatialGrid[ballRef](spatialCellSize),
		foods:   newSpatialGrid[*Food](spatialCellSize),
		ejected: newSpatialGrid[*EjectedMass](spatialCellSize),
		foodRadius: Radius(r.cfg.FoodMass, r.cfg.RadiusFactor),
	}

	for _, id := range r.order {
		p := r.players[id]
		if p == nil || !p.alive() {
			continue
		}
		for _, b := range p.Balls {
			idx.balls.Insert(b.X, b.Y, ballRef{owner: p, ball: b})
			if b.Radius > idx.maxBallRadius {
				idx.maxBallRadius = b.Radius
			}
		}
	}
	for _, f := range r.foods {
		idx.foods.Insert(f.X, f.Y, f)
	}
	for _, em := range r.ejected {
		idx.ejected.Insert(em.X, em.Y, em)
		if em.Radius > idx.maxEjectedRadius {
			idx.maxEjectedRadius = em.Radius
		}
	}
	return idx
}

// aoiViewForPlayerLocked 按设计文档的 MVP 策略计算视野：
// 多球体取中心点平均值，视野大小由最大球半径决定。
func (r *Room) aoiViewForPlayerLocked(p *Player) (aoiView, bool) {
	if p == nil || len(p.Balls) == 0 {
		return aoiView{}, false
	}

	var centerX, centerY, maxRadius float64
	for _, b := range p.Balls {
		centerX += b.X
		centerY += b.Y
		if b.Radius > maxRadius {
			maxRadius = b.Radius
		}
	}
	centerX /= float64(len(p.Balls))
	centerY /= float64(len(p.Balls))

	viewRadius := r.cfg.BaseViewRadius + maxRadius*r.cfg.ViewRadiusFactor
	if viewRadius < 0 {
		viewRadius = 0
	}
	if r.cfg.MaxViewRadius > 0 && viewRadius > r.cfg.MaxViewRadius {
		viewRadius = r.cfg.MaxViewRadius
	}
	return aoiView{centerX: centerX, centerY: centerY, radius: viewRadius}, true
}

// aoiSnapshotForPlayerLocked 生成当前玩家的完整可见状态。
// 当前玩家自身始终完整同步；远端玩家只携带进入 AOI 的球体。
func (r *Room) aoiSnapshotForPlayerLocked(
	viewer *Player,
	idx *roomAOIIndex,
	snapshotType string,
	now int64,
	events []protocol.SnapshotEvent,
) protocol.RoomSnapshotData {
	view, ok := r.aoiViewForPlayerLocked(viewer)
	if !ok {
		return protocol.RoomSnapshotData{
			RoomID: r.id, SnapshotType: snapshotType, TickSeq: r.tickSeq,
			ServerTime: now, Players: []protocol.SnapshotPlayer{}, Foods: []protocol.SnapshotFood{},
			Ejected: []protocol.SnapshotEjected{}, Events: events,
		}
	}

	visibleBallIDs := make(map[string]struct{})
	ballReach := view.radius + idx.maxBallRadius
	ballCandidates := idx.balls.QueryAABB(nil,
		view.centerX-ballReach, view.centerY-ballReach,
		view.centerX+ballReach, view.centerY+ballReach,
	)
	for _, candidate := range ballCandidates {
		if candidate.owner == viewer {
			continue
		}
		if intersectsAOI(view, candidate.ball.X, candidate.ball.Y, candidate.ball.Radius) {
			visibleBallIDs[candidate.ball.BallID] = struct{}{}
		}
	}

	players := make([]protocol.SnapshotPlayer, 0, len(r.order))
	for _, id := range r.order {
		p := r.players[id]
		if p == nil || !p.alive() {
			continue
		}
		balls := make([]protocol.Ball, 0, len(p.Balls))
		mass := 0.0
		for _, b := range p.Balls {
			if p != viewer {
				if _, visible := visibleBallIDs[b.BallID]; !visible {
					continue
				}
			}
			balls = append(balls, protocol.Ball{
				BallID: b.BallID, X: round1(b.X), Y: round1(b.Y),
				Radius: round1(b.Radius), Mass: round1(b.Mass),
			})
			mass += b.Mass
		}
		if len(balls) == 0 {
			continue
		}
		players = append(players, protocol.SnapshotPlayer{
			UserID: p.UserID, Nickname: p.Nickname, Status: p.Status,
			Score: int64(mass), Mass: round1(mass), Balls: balls,
		})
	}

	foodReach := view.radius + idx.foodRadius
	foodCandidates := idx.foods.QueryAABB(nil,
		view.centerX-foodReach, view.centerY-foodReach,
		view.centerX+foodReach, view.centerY+foodReach,
	)
	foods := make([]protocol.SnapshotFood, 0, len(foodCandidates))
	for _, f := range foodCandidates {
		if !intersectsAOI(view, f.X, f.Y, idx.foodRadius) {
			continue
		}
		foods = append(foods, protocol.SnapshotFood{
			FoodID: f.ID, X: round1(f.X), Y: round1(f.Y), Mass: f.Mass, Color: f.Color,
		})
	}
	sort.Slice(foods, func(i, j int) bool { return foods[i].FoodID < foods[j].FoodID })

	ejectedReach := view.radius + idx.maxEjectedRadius
	ejectedCandidates := idx.ejected.QueryAABB(nil,
		view.centerX-ejectedReach, view.centerY-ejectedReach,
		view.centerX+ejectedReach, view.centerY+ejectedReach,
	)
	ejected := make([]protocol.SnapshotEjected, 0, len(ejectedCandidates))
	for _, em := range ejectedCandidates {
		if !intersectsAOI(view, em.X, em.Y, em.Radius) {
			continue
		}
		ejected = append(ejected, protocol.SnapshotEjected{
			EjectID: em.ID, OwnerID: em.OwnerID,
			X: round1(em.X), Y: round1(em.Y), Radius: round1(em.Radius), Mass: em.Mass,
		})
	}
	sort.Slice(ejected, func(i, j int) bool { return ejected[i].EjectID < ejected[j].EjectID })

	return protocol.RoomSnapshotData{
		RoomID: r.id, SnapshotType: snapshotType, TickSeq: r.tickSeq,
		ServerTime: now, Players: players, Foods: foods, Ejected: ejected, Events: events,
	}
}

func intersectsAOI(view aoiView, x, y, objectRadius float64) bool {
	dx := x - view.centerX
	dy := y - view.centerY
	reach := view.radius + objectRadius
	return dx*dx+dy*dy <= reach*reach
}
