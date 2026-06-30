package game

import (
	"sort"
	"time"

	"cellular-phagocyte/server/internal/protocol"
)

const rankTopN = 10

// broadcastSnapshotLocked 构建并发送一份完整的 ROOM_SNAPSHOT，并清空累积的事件缓冲。
func (r *Room) broadcastSnapshotLocked() {
	now := time.Now().UnixMilli()
	r.snapshotSeq++

	players := make([]protocol.SnapshotPlayer, 0, len(r.order))
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		balls := make([]protocol.Ball, 0, len(p.Balls))
		for _, b := range p.Balls {
			balls = append(balls, protocol.Ball{
				BallID: b.BallID, X: round1(b.X), Y: round1(b.Y),
				Radius: round1(b.Radius), Mass: round1(b.Mass),
			})
		}
		mass := p.totalMass()
		players = append(players, protocol.SnapshotPlayer{
			UserID: p.UserID, Nickname: p.Nickname, Status: p.Status,
			Score: int64(mass), Mass: round1(mass), Balls: balls,
		})
	}

	foods := make([]protocol.SnapshotFood, 0, len(r.foods))
	for _, f := range r.foods {
		foods = append(foods, protocol.SnapshotFood{
			FoodID: f.ID, X: round1(f.X), Y: round1(f.Y), Mass: f.Mass, Color: f.Color,
		})
	}

	events := r.pendingEvents
	r.pendingEvents = nil

	env := protocol.Envelope{
		Type:       protocol.TypeRoomSnapshot,
		Seq:        r.snapshotSeq,
		ServerTime: now,
		Data: protocol.MustMarshal(protocol.RoomSnapshotData{
			RoomID: r.id, SnapshotType: "FULL", TickSeq: r.tickSeq,
			ServerTime: now, Players: players, Foods: foods,
			Ejected: r.ejectedSnapshotLocked(), Events: events,
		}),
	}
	r.broadcastLocked(env)
}

// ejectedSnapshotLocked 构建当前吐出物列表。
func (r *Room) ejectedSnapshotLocked() []protocol.SnapshotEjected {
	out := make([]protocol.SnapshotEjected, 0, len(r.ejected))
	for _, em := range r.ejected {
		out = append(out, protocol.SnapshotEjected{
			EjectID: em.ID, OwnerID: em.OwnerID,
			X: round1(em.X), Y: round1(em.Y), Radius: round1(em.Radius), Mass: em.Mass,
		})
	}
	return out
}

// recoverSnapshotLocked 构建用于重连恢复的全量快照（含全部玩家与对象）。
func (r *Room) recoverSnapshotLocked() protocol.RoomSnapshotData {
	players := make([]protocol.SnapshotPlayer, 0, len(r.order))
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		balls := make([]protocol.Ball, 0, len(p.Balls))
		for _, b := range p.Balls {
			balls = append(balls, protocol.Ball{
				BallID: b.BallID, X: round1(b.X), Y: round1(b.Y),
				Radius: round1(b.Radius), Mass: round1(b.Mass),
			})
		}
		mass := p.totalMass()
		players = append(players, protocol.SnapshotPlayer{
			UserID: p.UserID, Nickname: p.Nickname, Status: p.Status,
			Score: int64(mass), Mass: round1(mass), Balls: balls,
		})
	}
	foods := make([]protocol.SnapshotFood, 0, len(r.foods))
	for _, f := range r.foods {
		foods = append(foods, protocol.SnapshotFood{
			FoodID: f.ID, X: round1(f.X), Y: round1(f.Y), Mass: f.Mass, Color: f.Color,
		})
	}
	return protocol.RoomSnapshotData{
		RoomID: r.id, SnapshotType: "FULL_RECOVER", TickSeq: r.tickSeq,
		ServerTime: time.Now().UnixMilli(), Players: players, Foods: foods,
		Ejected: r.ejectedSnapshotLocked(), Events: nil,
	}
}

// rankedPlayersLocked 返回所有参与过对局的玩家，按 MaxMass 降序排列。
func (r *Room) rankedPlayersLocked() []*Player {
	list := make([]*Player, 0, len(r.order))
	for _, id := range r.order {
		p := r.players[id]
		if p.IsBot || p.Entered {
			list = append(list, p)
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].MaxMass != list[j].MaxMass {
			return list[i].MaxMass > list[j].MaxMass
		}
		return list[i].EatPlayerCount > list[j].EatPlayerCount
	})
	return list
}

// broadcastRankLocked 向每个已连接的真人玩家发送个性化的 RANK_UPDATE。
func (r *Room) broadcastRankLocked() {
	ranked := r.rankedPlayersLocked()

	topN := make([]protocol.RankEntry, 0, rankTopN)
	rankByUser := make(map[string]int, len(ranked))
	for i, p := range ranked {
		rankByUser[p.UserID] = i + 1
		if i < rankTopN {
			topN = append(topN, protocol.RankEntry{
				Rank: i + 1, UserID: p.UserID, Nickname: p.Nickname, Score: int64(p.MaxMass),
			})
		}
	}

	now := time.Now().UnixMilli()
	for _, id := range r.order {
		p := r.players[id]
		if p.conn == nil {
			continue
		}
		var self *protocol.SelfRank
		if rk, ok := rankByUser[p.UserID]; ok {
			self = &protocol.SelfRank{Rank: rk, Score: int64(p.MaxMass)}
		}
		p.conn.Send(protocol.Envelope{
			Type:       protocol.TypeRankUpdate,
			ServerTime: now,
			Data: protocol.MustMarshal(protocol.RankUpdateData{
				RoomID: r.id, RankTopN: topN, SelfRank: self,
			}),
		})
	}
}

func round1(v float64) float64 {
	return float64(int64(v*10)) / 10
}
