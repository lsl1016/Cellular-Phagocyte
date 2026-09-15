package game

import "cellular-phagocyte/server/internal/protocol"

// aoiView 是服务端为单个玩家计算出的圆形兴趣区域。
type aoiView struct {
	centerX float64
	centerY float64
	radius  float64
}

// roomAOIIndex 是一次快照周期内的只读空间索引。
// 它复用碰撞热路径已经落地的 spatialGrid 实现，但不会改变权威房间状态。
type roomAOIIndex struct {
	balls            *spatialGrid[ballRef]
	foods            *spatialGrid[*Food]
	ejected          *spatialGrid[*EjectedMass]
	ballCount        int
	maxBallRadius    float64
	maxEjectedRadius float64
	foodRadius       float64
}

func (r *Room) buildAOIIndexLocked() *roomAOIIndex {
	idx := &roomAOIIndex{
		balls:      newSpatialGrid[ballRef](spatialCellSize),
		foods:      newSpatialGrid[*Food](spatialCellSize),
		ejected:    newSpatialGrid[*EjectedMass](spatialCellSize),
		foodRadius: Radius(r.cfg.FoodMass, r.cfg.RadiusFactor),
	}

	for _, id := range r.order {
		p := r.players[id]
		if p == nil || !p.alive() {
			continue
		}
		for _, b := range p.Balls {
			idx.balls.Insert(b.X, b.Y, ballRef{owner: p, ball: b})
			idx.ballCount++
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

// aoiSnapshotScratch 在同一个快照周期内被所有 viewer 复用。
// JSON 必须在下一次 reset 前完成序列化；broadcastSnapshotLocked 正是按此顺序串行处理。
type aoiSnapshotScratch struct {
	visibleBalls      map[*Ball]struct{}
	ballCandidates    []ballRef
	foodCandidates    []*Food
	ejectedCandidates []*EjectedMass
	players           []protocol.SnapshotPlayer
	balls             []protocol.Ball
	foods             []protocol.SnapshotFood
	ejected           []protocol.SnapshotEjected
}

func newAOISnapshotScratch(playerCount, ballCount, foodCount, ejectedCount int) *aoiSnapshotScratch {
	return &aoiSnapshotScratch{
		visibleBalls:      make(map[*Ball]struct{}, ballCount),
		ballCandidates:    make([]ballRef, 0, ballCount),
		foodCandidates:    make([]*Food, 0, foodCount),
		ejectedCandidates: make([]*EjectedMass, 0, ejectedCount),
		players:           make([]protocol.SnapshotPlayer, 0, playerCount),
		balls:             make([]protocol.Ball, 0, ballCount),
		foods:             make([]protocol.SnapshotFood, 0, foodCount),
		ejected:           make([]protocol.SnapshotEjected, 0, ejectedCount),
	}
}

func (s *aoiSnapshotScratch) reset() {
	clear(s.visibleBalls)
	clear(s.ballCandidates)
	clear(s.foodCandidates)
	clear(s.ejectedCandidates)
	clear(s.players)
	clear(s.balls)
	clear(s.foods)
	clear(s.ejected)
	s.ballCandidates = s.ballCandidates[:0]
	s.foodCandidates = s.foodCandidates[:0]
	s.ejectedCandidates = s.ejectedCandidates[:0]
	s.players = s.players[:0]
	s.balls = s.balls[:0]
	s.foods = s.foods[:0]
	s.ejected = s.ejected[:0]
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

// aoiSnapshotForPlayerLocked 是单次调用包装器，供重连恢复和测试使用。
// 高频广播路径使用 aoiSnapshotForPlayerWithScratchLocked，在 viewer 之间复用 scratch。
func (r *Room) aoiSnapshotForPlayerLocked(
	viewer *Player,
	idx *roomAOIIndex,
	snapshotType string,
	now int64,
	events []protocol.SnapshotEvent,
) protocol.RoomSnapshotData {
	scratch := newAOISnapshotScratch(len(r.order), idx.ballCount, len(r.foods), len(r.ejected))
	return r.aoiSnapshotForPlayerWithScratchLocked(viewer, idx, scratch, snapshotType, now, events)
}

// aoiSnapshotForPlayerWithScratchLocked 生成当前玩家的完整可见状态。
// 当前玩家自身始终完整同步；远端玩家只携带进入 AOI 的球体。
// SnapshotPlayer 的聚合 Mass/Score 仍保持“玩家总质量”语义，不随分身进入/离开视野而跳变。
func (r *Room) aoiSnapshotForPlayerWithScratchLocked(
	viewer *Player,
	idx *roomAOIIndex,
	scratch *aoiSnapshotScratch,
	snapshotType string,
	now int64,
	events []protocol.SnapshotEvent,
) protocol.RoomSnapshotData {
	scratch.reset()
	view, ok := r.aoiViewForPlayerLocked(viewer)
	if !ok {
		return protocol.RoomSnapshotData{
			RoomID: r.id, SnapshotType: snapshotType, TickSeq: r.tickSeq,
			ServerTime: now, Players: scratch.players, Foods: scratch.foods,
			Ejected: scratch.ejected, Events: events,
		}
	}

	ballReach := view.radius + idx.maxBallRadius
	scratch.ballCandidates = idx.balls.QueryAABB(scratch.ballCandidates,
		view.centerX-ballReach, view.centerY-ballReach,
		view.centerX+ballReach, view.centerY+ballReach,
	)
	for _, candidate := range scratch.ballCandidates {
		if candidate.owner == viewer {
			continue
		}
		if intersectsAOI(view, candidate.ball.X, candidate.ball.Y, candidate.ball.Radius) {
			scratch.visibleBalls[candidate.ball] = struct{}{}
		}
	}

	for _, id := range r.order {
		p := r.players[id]
		if p == nil || !p.alive() {
			continue
		}
		start := len(scratch.balls)
		for _, b := range p.Balls {
			if p != viewer {
				if _, visible := scratch.visibleBalls[b]; !visible {
					continue
				}
			}
			scratch.balls = append(scratch.balls, protocol.Ball{
				BallID: b.BallID, X: round1(b.X), Y: round1(b.Y),
				Radius: round1(b.Radius), Mass: round1(b.Mass),
			})
		}
		if start == len(scratch.balls) {
			continue
		}
		mass := p.totalMass()
		scratch.players = append(scratch.players, protocol.SnapshotPlayer{
			UserID: p.UserID, Nickname: p.Nickname, Status: p.Status,
			Score: int64(mass), Mass: round1(mass), Balls: scratch.balls[start:len(scratch.balls)],
		})
	}

	foodReach := view.radius + idx.foodRadius
	scratch.foodCandidates = idx.foods.QueryAABB(scratch.foodCandidates,
		view.centerX-foodReach, view.centerY-foodReach,
		view.centerX+foodReach, view.centerY+foodReach,
	)
	for _, f := range scratch.foodCandidates {
		if !intersectsAOI(view, f.X, f.Y, idx.foodRadius) {
			continue
		}
		scratch.foods = append(scratch.foods, protocol.SnapshotFood{
			FoodID: f.ID, X: round1(f.X), Y: round1(f.Y), Mass: f.Mass, Color: f.Color,
		})
	}

	ejectedReach := view.radius + idx.maxEjectedRadius
	scratch.ejectedCandidates = idx.ejected.QueryAABB(scratch.ejectedCandidates,
		view.centerX-ejectedReach, view.centerY-ejectedReach,
		view.centerX+ejectedReach, view.centerY+ejectedReach,
	)
	for _, em := range scratch.ejectedCandidates {
		if !intersectsAOI(view, em.X, em.Y, em.Radius) {
			continue
		}
		scratch.ejected = append(scratch.ejected, protocol.SnapshotEjected{
			EjectID: em.ID, OwnerID: em.OwnerID,
			X: round1(em.X), Y: round1(em.Y), Radius: round1(em.Radius), Mass: em.Mass,
		})
	}

	return protocol.RoomSnapshotData{
		RoomID: r.id, SnapshotType: snapshotType, TickSeq: r.tickSeq,
		ServerTime: now, Players: scratch.players, Foods: scratch.foods, Ejected: scratch.ejected, Events: events,
	}
}

func intersectsAOI(view aoiView, x, y, objectRadius float64) bool {
	dx := x - view.centerX
	dy := y - view.centerY
	reach := view.radius + objectRadius
	return dx*dx+dy*dy <= reach*reach
}
