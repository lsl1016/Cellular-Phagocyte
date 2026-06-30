package game

import (
	"math"
	"time"

	"cellular-phagocyte/server/internal/idgen"
	"cellular-phagocyte/server/internal/protocol"
)

// 技能失败原因。
const (
	reasonRoomNotRunning   = "ROOM_NOT_RUNNING"
	reasonPlayerNotPlaying = "PLAYER_NOT_PLAYING"
	reasonMassNotEnough    = "MASS_NOT_ENOUGH"
	reasonSplitLimit       = "SPLIT_LIMIT_REACHED"
	reasonSplitCooldown    = "SPLIT_COOLDOWN"
	reasonEjectCooldown    = "EJECT_COOLDOWN"
)

// Split 处理分裂请求：把质量最大的球分裂出一个带冲量的新球。
func (r *Room) Split(userID string, dir float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.players[userID]
	now := time.Now().UnixMilli()
	if r.status != RoomRunning {
		r.skillFailLocked(p, protocol.TypeSplit, reasonRoomNotRunning, "房间未在对战中")
		return
	}
	if p == nil || !p.alive() {
		r.skillFailLocked(p, protocol.TypeSplit, reasonPlayerNotPlaying, "玩家不可操作")
		return
	}
	if len(p.Balls) >= r.cfg.MaxSplitBalls {
		r.skillFailLocked(p, protocol.TypeSplit, reasonSplitLimit, "分身数量已达上限")
		return
	}
	if now < p.nextSplitTime {
		r.skillFailLocked(p, protocol.TypeSplit, reasonSplitCooldown, "分裂冷却中")
		return
	}
	src := r.largestBall(p)
	if src == nil || src.Mass < r.cfg.MinSplitMass {
		r.skillFailLocked(p, protocol.TypeSplit, reasonMassNotEnough, "质量不足，无法分裂")
		return
	}

	newMass := src.Mass * r.cfg.SplitMassRatio
	src.Mass -= newMass
	src.Radius = Radius(src.Mass, r.cfg.RadiusFactor)
	mergeAt := now + int64(r.cfg.MergeDelaySeconds)*1000
	src.canMergeAt = mergeAt

	offset := src.Radius * 0.8
	nb := &Ball{
		BallID:     "b_" + idgen.Short(),
		X:          src.X + math.Cos(dir)*offset,
		Y:          src.Y + math.Sin(dir)*offset,
		Mass:       newMass,
		Radius:     Radius(newMass, r.cfg.RadiusFactor),
		vx:         math.Cos(dir) * r.cfg.SplitBoostSpeed,
		vy:         math.Sin(dir) * r.cfg.SplitBoostSpeed,
		boostUntil: now + r.cfg.SplitBoostDurationMs,
		canMergeAt: mergeAt,
	}
	p.Balls = append(p.Balls, nb)
	p.nextSplitTime = now + r.cfg.SplitCooldownMs

	r.addEvent("PLAYER_SPLIT", map[string]any{
		"userId": p.UserID, "sourceBallId": src.BallID, "newBallId": nb.BallID,
		"direction": dir, "newMass": newMass,
	})
}

// Eject 处理吐球请求：扣除质量并生成一个带初速度的吐出物。
func (r *Room) Eject(userID string, dir float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.players[userID]
	now := time.Now().UnixMilli()
	if r.status != RoomRunning {
		r.skillFailLocked(p, protocol.TypeEject, reasonRoomNotRunning, "房间未在对战中")
		return
	}
	if p == nil || !p.alive() {
		r.skillFailLocked(p, protocol.TypeEject, reasonPlayerNotPlaying, "玩家不可操作")
		return
	}
	src := r.largestBall(p)
	if src == nil || src.Mass < r.cfg.MinEjectMass {
		r.skillFailLocked(p, protocol.TypeEject, reasonMassNotEnough, "质量不足，无法吐球")
		return
	}
	if now < p.nextEjectTime {
		r.skillFailLocked(p, protocol.TypeEject, reasonEjectCooldown, "吐球冷却中")
		return
	}
	if now-p.ejectWindowFrom >= 1000 {
		p.ejectWindowFrom = now
		p.ejectInWindow = 0
	}
	if p.ejectInWindow >= r.cfg.MaxEjectPerSec {
		r.skillFailLocked(p, protocol.TypeEject, reasonEjectCooldown, "吐球过于频繁")
		return
	}

	src.Mass -= r.cfg.EjectMass
	src.Radius = Radius(src.Mass, r.cfg.RadiusFactor)

	emMass := r.cfg.EjectMass
	emRadius := Radius(emMass, r.cfg.RadiusFactor)
	offset := src.Radius + emRadius
	em := &EjectedMass{
		ID:           "ej_" + idgen.Short(),
		OwnerID:      p.UserID,
		X:            src.X + math.Cos(dir)*offset,
		Y:            src.Y + math.Sin(dir)*offset,
		Mass:         emMass,
		Radius:       emRadius,
		vx:           math.Cos(dir) * r.cfg.EjectSpeed,
		vy:           math.Sin(dir) * r.cfg.EjectSpeed,
		moveUntil:    now + r.cfg.EjectMoveMs,
		protectUntil: now + 300,
	}
	r.ejected[em.ID] = em
	p.nextEjectTime = now + r.cfg.EjectIntervalMs
	p.ejectInWindow++

	r.addEvent("PLAYER_EJECT", map[string]any{
		"userId": p.UserID, "sourceBallId": src.BallID, "ejectId": em.ID,
		"direction": dir, "ejectMass": emMass,
	})
}

func (r *Room) largestBall(p *Player) *Ball {
	var best *Ball
	for _, b := range p.Balls {
		if best == nil || b.Mass > best.Mass {
			best = b
		}
	}
	return best
}

func (r *Room) skillFailLocked(p *Player, skillType, reason, message string) {
	if p == nil || p.conn == nil {
		return
	}
	p.conn.Send(protocol.Envelope{
		Type:       protocol.TypeSkillFailed,
		ServerTime: time.Now().UnixMilli(),
		Data: protocol.MustMarshal(protocol.SkillFailedData{
			SkillType: skillType, Reason: reason, Message: message,
		}),
	})
}
