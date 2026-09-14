package game

import (
	"time"

	"cellular-phagocyte/server/internal/protocol"
)

// finishedRoomGrace 给已经完成结算但客户端恰好断线的连接留出补发窗口。
// 客户端当前重连退避为 0s/1s/2s/...，5 秒足以覆盖前几次恢复尝试，同时不会长期占用房间内存。
const finishedRoomGrace = 5 * time.Second

// endGame 冻结房间、广播 GAME_END、执行结算、向每个玩家推送 SETTLEMENT_RESULT，
// 最后安排资源清理。结算结果会在 finishedRoomGrace 内保留，供极端慢客户端断线后重连补发。
func (r *Room) endGame(reason string) {
	r.mu.Lock()
	if r.finished {
		r.mu.Unlock()
		return
	}
	r.finished = true
	r.status = RoomSettling
	r.endReason = reason
	req := r.buildSettleRequestLocked()
	now := time.Now().UnixMilli()
	r.broadcastLocked(protocol.Envelope{
		Type:       protocol.TypeGameEnd,
		ServerTime: now,
		Data: protocol.MustMarshal(protocol.GameEndData{
			RoomID: r.id, Reason: reason, Message: "对局结束，正在结算...",
		}),
	})
	r.mu.Unlock()

	r.log.Info("room_game_end", "roomId", r.id, "reason", reason, "players", req.TotalPlayers)

	var results []protocol.SettlementResultData
	if r.mgr.settler != nil {
		results = r.mgr.settler.Settle(req)
	}

	r.mu.Lock()
	st := time.Now().UnixMilli()
	for _, res := range results {
		r.settlementResults[res.UserID] = res
		if p, ok := r.players[res.UserID]; ok && p.conn != nil {
			p.conn.Send(protocol.Envelope{
				Type:       protocol.TypeSettlementResult,
				ServerTime: st,
				Data:       protocol.MustMarshal(res),
			})
		}
	}
	// 必须在 settlementResults 全部写入之后再暴露 RoomFinished，
	// 这样 finished reconnect 一旦成功就一定能读取到稳定的最终结果。
	r.status = RoomFinished
	r.mu.Unlock()

	go func() {
		time.Sleep(finishedRoomGrace)
		r.mu.Lock()
		for _, id := range r.order {
			if p := r.players[id]; p.conn != nil {
				p.conn.Close()
			}
		}
		r.mu.Unlock()
		r.mgr.removeRoom(r.id)
	}()
}

// endImmediately 销毁一个从未有人入房的房间。
func (r *Room) endImmediately() {
	r.mu.Lock()
	if r.finished {
		r.mu.Unlock()
		return
	}
	r.finished = true
	r.status = RoomFinished
	r.mu.Unlock()
	r.mgr.removeRoom(r.id)
}

// buildSettleRequestLocked 计算最终排名并生成每个玩家的结果。
func (r *Room) buildSettleRequestLocked() *SettleRequest {
	ranked := r.rankedPlayersLocked()
	tickRate := r.cfg.TickRate
	if tickRate < 1 {
		tickRate = 1
	}

	players := make([]SettlePlayerResult, 0, len(ranked))
	for i, p := range ranked {
		players = append(players, SettlePlayerResult{
			UserID:         p.UserID,
			Nickname:       p.Nickname,
			IsBot:          p.IsBot,
			Rank:           i + 1,
			FinalScore:     int64(p.MaxMass),
			MaxMass:        int64(p.MaxMass),
			EatPlayerCount: p.EatPlayerCount,
			EatFoodCount:   p.EatFoodCount,
			AliveSeconds:   int(p.aliveTicks) / tickRate,
			Alive:          p.alive(),
		})
	}

	return &SettleRequest{
		RoomID:                r.id,
		MatchID:               r.matchID,
		Mode:                  r.mode,
		StartTime:             r.startTimeMs,
		EndTime:               time.Now().UnixMilli(),
		BattleDurationSeconds: r.cfg.BattleDurationSeconds,
		TotalPlayers:          len(players),
		Players:               players,
	}
}
