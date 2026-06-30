package game

import (
	"math"
	"time"

	"cellular-phagocyte/server/internal/protocol"
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

	r.applyInputsLocked()
	r.botAILocked()
	r.moveLocked(dt)
	r.eatFoodLocked()
	r.eatPlayersLocked()
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

	if time.Now().UnixMilli() >= r.endTimeMs {
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

// botAILocked 让每个机器人朝最近的食物移动。
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

func (r *Room) moveLocked(dt float64) {
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		for _, b := range p.Balls {
			speed := Speed(r.cfg.BaseSpeed, b.Mass, r.cfg.PlayerInitialMass)
			b.X += math.Cos(p.Direction) * speed * dt
			b.Y += math.Sin(p.Direction) * speed * dt
			b.X = clampF(b.X, b.Radius, r.cfg.MapWidth-b.Radius)
			b.Y = clampF(b.Y, b.Radius, r.cfg.MapHeight-b.Radius)
		}
	}
}

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
					b.Mass += f.Mass
					b.Radius = Radius(b.Mass, r.cfg.RadiusFactor)
					delete(r.foods, fid)
					p.EatFoodCount++
					r.addEvent("FOOD_EATEN", map[string]any{
						"userId": p.UserID, "ballId": b.BallID, "foodId": fid,
						"gainMass": f.Mass, "newMass": b.Mass,
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
				big.owner.EatPlayerCount++
				eaten[small.ball.BallID] = true
				r.killPlayerLocked(small.owner, big.owner.UserID, gain, big.ball.Mass)
			}
		}
	}
}

func (r *Room) killPlayerLocked(victim *Player, attackerID string, gain, attackerNewMass float64) {
	r.addEvent("PLAYER_EATEN", map[string]any{
		"attackerUserId": attackerID, "targetUserId": victim.UserID,
		"gainMass": gain, "attackerNewMass": attackerNewMass, "targetDead": true,
	})
	r.addEvent("PLAYER_DEAD", map[string]any{"userId": victim.UserID})
	victim.dead = true
	victim.Balls = nil
	victim.Status = StatusDead
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

func (r *Room) addEvent(t string, data map[string]any) {
	r.pendingEvents = append(r.pendingEvents, protocol.SnapshotEvent{
		Type: t, Data: protocol.MustMarshal(data),
	})
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
