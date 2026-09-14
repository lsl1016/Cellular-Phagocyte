package game

import (
	"testing"

	"cellular-phagocyte/server/internal/config"
)

// TestEatPlayersKeepsVictimAliveWithRemainingBalls 回归验证：
// 分裂玩家只丢失被吞噬的那个球，直到最后一个球被吃才真正死亡。
func TestEatPlayersKeepsVictimAliveWithRemainingBalls(t *testing.T) {
	cfg := config.Default().Game
	r := newRoom("room_test", "match_test", "classic", cfg, nil, nil)

	attacker := &Player{
		UserID: "attacker",
		Status: StatusPlaying,
		Balls: []*Ball{{
			BallID: "attacker_ball",
			X:      100,
			Y:      100,
			Mass:   100,
			Radius: Radius(100, cfg.RadiusFactor),
		}},
	}
	victim := &Player{
		UserID: "victim",
		Status: StatusPlaying,
		Balls: []*Ball{
			{
				BallID: "victim_ball_1",
				X:      100,
				Y:      100,
				Mass:   10,
				Radius: Radius(10, cfg.RadiusFactor),
			},
			{
				BallID: "victim_ball_2",
				X:      1000,
				Y:      1000,
				Mass:   10,
				Radius: Radius(10, cfg.RadiusFactor),
			},
		},
	}

	r.players[attacker.UserID] = attacker
	r.players[victim.UserID] = victim
	r.order = []string{attacker.UserID, victim.UserID}

	r.eatPlayersLocked()

	if victim.dead {
		t.Fatal("victim should remain alive while one split ball is still alive")
	}
	if victim.Status != StatusPlaying {
		t.Fatalf("victim status = %q, want %q", victim.Status, StatusPlaying)
	}
	if len(victim.Balls) != 1 || victim.Balls[0].BallID != "victim_ball_2" {
		t.Fatalf("remaining balls = %+v, want only victim_ball_2", victim.Balls)
	}
	if attacker.EatPlayerCount != 0 {
		t.Fatalf("EatPlayerCount = %d, want 0 before eliminating the player", attacker.EatPlayerCount)
	}

	// 把最后一个分身移到攻击者中心，第二次吞噬才应判定玩家死亡。
	victim.Balls[0].X = attacker.Balls[0].X
	victim.Balls[0].Y = attacker.Balls[0].Y
	r.eatPlayersLocked()

	if !victim.dead || victim.Status != StatusDead || len(victim.Balls) != 0 {
		t.Fatalf("victim should be dead after last ball is eaten: dead=%v status=%q balls=%d", victim.dead, victim.Status, len(victim.Balls))
	}
	if attacker.EatPlayerCount != 1 {
		t.Fatalf("EatPlayerCount = %d, want 1 after eliminating the player", attacker.EatPlayerCount)
	}
}
