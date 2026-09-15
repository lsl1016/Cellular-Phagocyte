package game

import (
	"sort"
	"time"

	"cellular-phagocyte/server/internal/protocol"
)

const rankTopN = 10

// broadcastSnapshotLocked 构建 ROOM_SNAPSHOT，并清空累积的事件缓冲。
// AOI 开启时：首帧/周期校正/FULL_SYNC 发送 AOI_FULL，其余帧发送 AOI_DELTA；
// AOI 关闭时保留旧的全房间 FULL 广播路径。所有高频状态都走 latest-only snapshot lane。
func (r *Room) broadcastSnapshotLocked() {
	now := time.Now().UnixMilli()
	r.snapshotSeq++
	events := r.pendingEvents
	routes := r.pendingEventRoutes
	r.pendingEvents = nil
	r.pendingEventRoutes = nil

	if r.cfg.AOIEnabled {
		idx := r.buildAOIIndexLocked()
		scratch := newAOISnapshotScratch(len(r.order), idx.ballCount, len(r.foods), len(r.ejected))
		for _, id := range r.order {
			p := r.players[id]
			if p == nil || p.conn == nil {
				continue
			}

			state := r.aoiStateLocked(p.UserID)
			viewerEvents := r.eventsForViewerLocked(p, state, events, routes)

			// full 的 slice 引用 scratch；必须在下一位 viewer reset scratch 前完成 Marshal。
			// 事件必须在 aoiDeltaFromFullLocked 推进 VisibleSet 之前过滤，
			// 这样“上一帧可见、本帧已删除”的对象仍能把事件送达观察者。
			full := r.aoiSnapshotForPlayerWithScratchLocked(p, idx, scratch, protocol.SnapshotAOIFull, now, viewerEvents)
			full.SnapshotSeq = r.snapshotSeq

			var data []byte
			if r.shouldSendAOIFullLocked(state) {
				data = protocol.MustMarshal(full)
				r.rememberAOIFullLocked(state, full, true)
			} else {
				// aoiDeltaFromFullLocked 会在比较的同时把 visible state 原地推进到当前 snapshotSeq，
				// 避免再清空/重建一轮 per-client map。
				delta := r.aoiDeltaFromFullLocked(state, full)
				data = protocol.MustMarshal(delta)
			}

			p.conn.SendSnapshot(protocol.Envelope{
				Type:       protocol.TypeRoomSnapshot,
				Seq:        r.snapshotSeq,
				ServerTime: now,
				Data:       data,
			})
		}
		return
	}

	// AOI 关闭时保持旧行为：所有事件仍随全房间 FULL 发送给每个连接。
	data := r.fullSnapshotDataLocked(protocol.SnapshotFull, now, events)
	env := protocol.Envelope{
		Type:       protocol.TypeRoomSnapshot,
		Seq:        r.snapshotSeq,
		ServerTime: now,
		Data:       protocol.MustMarshal(data),
	}
	for _, id := range r.order {
		p := r.players[id]
		if p != nil && p.conn != nil {
			p.conn.SendSnapshot(env)
		}
	}
}

// fullSnapshotDataLocked 构建旧版全房间状态，用于关闭 AOI 时的兼容路径。
func (r *Room) fullSnapshotDataLocked(snapshotType string, now int64, events []protocol.SnapshotEvent) protocol.RoomSnapshotData {
	players := make([]protocol.SnapshotPlayer, 0, len(r.order))
	for _, id := range r.order {
		p := r.players[id]
		if p == nil || !p.alive() {
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
		RoomID: r.id, SnapshotType: snapshotType, SnapshotSeq: r.snapshotSeq, TickSeq: r.tickSeq,
		ServerTime: now, Players: players, Foods: foods,
		Ejected: r.ejectedSnapshotLocked(), Events: events,
	}
}

// ejectedSnapshotLocked 构建当前全部吐出物列表，仅供非 AOI 全量路径使用。
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

// recoverSnapshotLocked 构建重连恢复快照。
// AOI 开启时恢复当前玩家视野，并把该 FULL 记为新连接后续 DELTA 的基线。
func (r *Room) recoverSnapshotLocked(userID string) protocol.RoomSnapshotData {
	now := time.Now().UnixMilli()
	if r.cfg.AOIEnabled {
		if p := r.players[userID]; p != nil {
			full := r.aoiSnapshotForPlayerLocked(p, r.buildAOIIndexLocked(), protocol.SnapshotAOIRecover, now, nil)
			full.SnapshotSeq = r.snapshotSeq
			r.rememberAOIFullLocked(r.aoiStateLocked(userID), full, true)
			return full
		}
	}
	return r.fullSnapshotDataLocked(protocol.SnapshotFullRecover, now, nil)
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
