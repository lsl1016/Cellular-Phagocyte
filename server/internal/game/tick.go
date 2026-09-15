package game

import (
	"math"
	"time"
)

// tickLoop 以配置的 Tick 频率驱动房间，直到对局结束。
func (r *Room) tickLoop() {
	interval := time.Second / time.Duration(r.cfg.TickRate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	snapshotEvery := r.cfg.TickRate / r.cfg.SnapshotRate
	if snapshotEvery < 1 {
		snapshotEvery = 1
	}
	rankEvery := r.cfg.TickRate / r.cfg.RankUpdateRate
	if rankEvery < 1 {
		rankEvery = 1
	}

	for range ticker.C {
		r.mu.Lock()
		ended := r.stepLocked(snapshotEvery, rankEvery)
		r.mu.Unlock()
		if ended {
			r.endGame("TIME_LIMIT")
			return
		}
	}
}

// stepLocked 推进房间一个 Tick。调用方需持有 r.mu。返回 true 表示对局应结束。
func (r *Room) stepLocked(snapshotEvery, rankEvery int) bool {
	if r.finished {
		return true
	}
	r.tickSeq++
	dt := 1.0 / float64(r.cfg.TickRate)
	now := time.Now().UnixMilli()

	r.applyInputsLocked()
	r.botAISpatialLocked()
	r.moveLocked(dt, now)
	r.updateEjectedLocked(dt)
	r.eatFoodSpatialLocked()
	r.eatEjectedSpatialLocked(now)
	r.eatPlayersSpatialLocked()
	r.mergeBallsLocked(now)
	r.checkReconnectTimeoutLocked(now)
	r.updateStatsLocked()

	if r.tickSeq%int64(r.cfg.TickRate) == 0 {
		r.replenishFoodLocked()
	}
	if r.tickSeq%int64(snapshotEvery) == 0 {
		r.broadcastSnapshotLocked()
	}
	if r.tickSeq%int64(rankEvery) == 0 {
		r.broadcastRankLocked()
	}

	if now >= r.endTimeMs {
		return true
	}
	return r.allHumansFinishedLocked()
}

func (r *Room) applyInputsLocked() {
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		if p.pendingDir != nil {
			p.Direction = *p.pendingDir
			p.pendingDir = nil
		}
	}
}

// botAILocked 是原始全量扫描实现，保留用于行为回归测试。
func (r *Room) botAILocked() {
	for _, id := range r.order {
		p := r.players[id]
		if !p.IsBot || !p.alive() {
			continue
		}
		b := p.Balls[0]
		var best *Food
		bestDist := math.MaxFloat64
		for _, f := range r.foods {
			d := Distance(b.X, b.Y, f.X, f.Y)
			if d < bestDist {
				bestDist = d
				best = f
			}
		}
		if best != nil {
			p.Direction = math.Atan2(best.Y-b.Y, best.X-b.X)
		}
	}
}

func (r *Room) moveLocked(dt float64, now int64) {
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		// 断线玩家停止移动（STOP 策略）
		if p.Status == StatusDisconnected {
			continue
		}
		dirX := math.Cos(p.Direction)
		dirY := math.Sin(p.Direction)
		for _, b := range p.Balls {
			speed := Speed(r.cfg.BaseSpeed, b.Mass, r.cfg.PlayerInitialMass)
			vx := dirX * speed
			vy := dirY * speed
			// 分裂冲量随剩余时间线性衰减叠加
			if b.boostUntil > now {
				frac := float64(b.boostUntil-now) / float64(r.cfg.SplitBoostDurationMs)
				vx += b.vx * frac
				vy += b.vy * frac
			}
			b.X = clampF(b.X+vx*dt, b.Radius, r.cfg.MapWidth-b.Radius)
			b.Y = clampF(b.Y+vy*dt, b.Radius, r.cfg.MapHeight-b.Radius)
		}
	}
}

// updateEjectedLocked 推进吐出物飞行；超时则停留为可吞噬对象。
func (r *Room) updateEjectedLocked(dt float64) {
	now := time.Now().UnixMilli()
	for _, em := range r.ejected {
		if now < em.moveUntil {
			em.X = clampF(em.X+em.vx*dt, em.Radius, r.cfg.MapWidth-em.Radius)
			em.Y = clampF(em.Y+em.vy*dt, em.Radius, r.cfg.MapHeight-em.Radius)
		}
	}
}

// eatEjectedLocked 是原始全量扫描实现，保留用于行为回归测试。
func (r *Room) eatEjectedLocked(now int64) {
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		for _, b := range p.Balls {
			for eid, em := range r.ejected {
				if em.OwnerID == p.UserID && now < em.protectUntil {
					continue // 保护期内原主不可吃回
				}
				if CanEatFood(b.Radius, em.Radius, Distance(b.X, b.Y, em.X, em.Y)) {
					eventX, eventY, eventRadius := em.X, em.Y, em.Radius
					ownerID := em.OwnerID
					b.Mass += em.Mass * r.cfg.EjectGainRatio
					b.Radius = Radius(b.Mass, r.cfg.RadiusFactor)
					delete(r.ejected, eid)
					r.addRoutedEvent("EJECTED_MASS_EATEN", map[string]any{
						"userId": p.UserID, "ballId": b.BallID, "ejectId": eid, "gainMass": em.Mass,
					}, snapshotEventRoute{
						ParticipantUserIDs: []string{p.UserID, ownerID},
						PlayerIDs:          []string{p.UserID, ownerID},
						EjectedIDs:         []string{eid},
						HasPosition: true, X: eventX, Y: eventY, Radius: eventRadius,
					})
				}
			}
		}
	}
}

// mergeBallsLocked 处理同一玩家到期且重叠的分身合体。
func (r *Room) mergeBallsLocked(now int64) {
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() || len(p.Balls) < 2 {
			continue
		}
		merged := true
		for merged {
			merged = false
			for i := 0; i < len(p.Balls); i++ {
				for j := i + 1; j < len(p.Balls); j++ {
					a, b := p.Balls[i], p.Balls[j]
					if now < a.canMergeAt || now < b.canMergeAt {
						continue
					}
					bigger, smaller := a, b
					if b.Mass > a.Mass {
						bigger, smaller = b, a
					}
					if Distance(a.X, a.Y, b.X, b.Y) > bigger.Radius {
						continue
					}
					bigger.Mass += smaller.Mass
					bigger.Radius = Radius(bigger.Mass, r.cfg.RadiusFactor)
					p.Balls = removeBall(p.Balls, smaller)
					r.addRoutedEvent("PLAYER_MERGE", map[string]any{"userId": p.UserID, "ballId": bigger.BallID}, snapshotEventRoute{
						ParticipantUserIDs: []string{p.UserID},
						PlayerIDs:          []string{p.UserID},
						HasPosition: true, X: bigger.X, Y: bigger.Y, Radius: bigger.Radius,
					})
					merged = true
					break
				}
				if merged {
					break
				}
			}
		}
	}
}

func removeBall(balls []*Ball, target *Ball) []*Ball {
	out := balls[:0]
	for _, b := range balls {
		if b != target {
			out = append(out, b)
		}
	}
	return out
}

// checkReconnectTimeoutLocked 将超过重连窗口仍未回来的玩家判负。
func (r *Room) checkReconnectTimeoutLocked(now int64) {
	for _, id := range r.order {
		p := r.players[id]
		if p.Status == StatusDisconnected && p.disconnectDeadline != 0 && now > p.disconnectDeadline {
			p.dead = true
			p.Balls = nil
			p.Status = StatusExited
			p.disconnectDeadline = 0
			r.addRoutedEvent("PLAYER_DEAD", map[string]any{"userId": p.UserID, "reason": "RECONNECT_TIMEOUT"}, snapshotEventRoute{
				ParticipantUserIDs: []string{p.UserID},
				PlayerIDs:          []string{p.UserID},
			})
		}
	}
}

// eatFoodLocked 是原始全量扫描实现，保留用于行为回归测试。
func (r *Room) eatFoodLocked() {
	foodRadius := Radius(r.cfg.FoodMass, r.cfg.RadiusFactor)
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		for _, b := range p.Balls {
			for fid, f := range r.foods {
				if CanEatFood(b.Radius, foodRadius, Distance(b.X, b.Y, f.X, f.Y)) {
					eventX, eventY := f.X, f.Y
					b.Mass += f.Mass
					b.Radius = Radius(b.Mass, r.cfg.RadiusFactor)
					delete(r.foods, fid)
					p.EatFoodCount++
					r.addRoutedEvent("FOOD_EATEN", map[string]any{
						"userId": p.UserID, "ballId": b.BallID, "foodId": fid,
						"gainMass": f.Mass, "newMass": b.Mass,
					}, snapshotEventRoute{
						ParticipantUserIDs: []string{p.UserID},
						PlayerIDs:          []string{p.UserID},
						FoodIDs:            []string{fid},
						HasPosition: true, X: eventX, Y: eventY, Radius: foodRadius,
					})
				}
			}
		}
	}
}

type ballRef struct {
	owner *Player
	ball  *Ball
}

// eatPlayersLocked 是原始全量两两比较实现，保留用于行为回归测试。
func (r *Room) eatPlayersLocked() {
	var balls []ballRef
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		for _, b := range p.Balls {
			balls = append(balls, ballRef{owner: p, ball: b})
		}
	}

	eaten := make(map[string]bool)
	for i := 0; i < len(balls); i++ {
		for j := i + 1; j < len(balls); j++ {
			a, t := balls[i], balls[j]
			if a.owner == t.owner || eaten[a.ball.BallID] || eaten[t.ball.BallID] {
				continue
			}
			// 调整方向，使 big 始终是较重的球
			big, small := a, t
			if t.ball.Mass > a.ball.Mass {
				big, small = t, a
			}
			dist := Distance(big.ball.X, big.ball.Y, small.ball.X, small.ball.Y)
			if CanEatPlayer(big.ball.Mass, small.ball.Mass, big.ball.Radius, small.ball.Radius, dist, r.cfg.EatMassRatio, r.cfg.EatDepthFactor) {
				gain := PlayerEatGain(small.ball.Mass, r.cfg.PlayerEatMassGain)
				big.ball.Mass += gain
				big.ball.Radius = Radius(big.ball.Mass, r.cfg.RadiusFactor)
				eaten[small.ball.BallID] = true
				if r.eatPlayerBallLocked(small.owner, small.ball, big.owner.UserID, gain, big.ball.Mass) {
					big.owner.EatPlayerCount++
				}
			}
		}
	}
}

// eatPlayerBallLocked 只移除被吞噬的单个分身；仅当最后一个球被吞噬时才判定玩家死亡。
// 返回 true 表示本次吞噬淘汰了目标玩家。
func (r *Room) eatPlayerBallLocked(victim *Player, victimBall *Ball, attackerID string, gain, attackerNewMass float64) bool {
	eventX, eventY, eventRadius := victimBall.X, victimBall.Y, victimBall.Radius
	victim.Balls = removeBall(victim.Balls, victimBall)
	targetDead := len(victim.Balls) == 0
	route := snapshotEventRoute{
		ParticipantUserIDs: []string{attackerID, victim.UserID},
		PlayerIDs:          []string{attackerID, victim.UserID},
		HasPosition: true, X: eventX, Y: eventY, Radius: eventRadius,
	}

	r.addRoutedEvent("PLAYER_EATEN", map[string]any{
		"attackerUserId": attackerID, "targetUserId": victim.UserID,
		"targetBallId": victimBall.BallID, "gainMass": gain,
		"attackerNewMass": attackerNewMass, "targetDead": targetDead,
	}, route)

	if !targetDead {
		return false
	}

	r.addRoutedEvent("PLAYER_DEAD", map[string]any{"userId": victim.UserID}, route)
	victim.dead = true
	victim.Status = StatusDead
	return true
}

func (r *Room) updateStatsLocked() {
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		p.aliveTicks++
		if m := p.totalMass(); m > p.MaxMass {
			p.MaxMass = m
		}
	}
}

func (r *Room) replenishFoodLocked() {
	for len(r.foods) < r.cfg.MaxFoodCount {
		r.spawnOneFood()
	}
}

// allHumansFinishedLocked 在至少有一个真人玩过且当前没有真人存活时返回 true，
// 用于提前结束对局而不必等到计时结束。
func (r *Room) allHumansFinishedLocked() bool {
	played, alive := false, 0
	for _, id := range r.order {
		p := r.players[id]
		if p.IsBot || !p.Entered {
			continue
		}
		played = true
		if p.alive() {
			alive++
		}
	}
	return played && alive == 0
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
