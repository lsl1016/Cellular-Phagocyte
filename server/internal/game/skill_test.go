package game

import (
	"io"
	"log/slog"
	"testing"

	"cellular-phagocyte/server/internal/config"
)

func testRoom(t *testing.T) *Room {
	t.Helper()
	cfg := config.Default()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := NewManager(cfg, nil, log, NewMemoryTokenStore())
	r := newRoom("r_test", "m_test", "classic", cfg.Game, mgr, log)
	r.status = RoomRunning
	return r
}

func addPlayingHuman(r *Room, userID string, mass float64) *Player {
	p := &Player{UserID: userID, Nickname: userID, Status: StatusPlaying, Entered: true}
	p.Balls = []*Ball{{BallID: "b_" + userID, X: 1000, Y: 1000, Mass: mass, Radius: Radius(mass, r.cfg.RadiusFactor)}}
	r.players[userID] = p
	r.order = append(r.order, userID)
	return p
}

func TestSplitCreatesSecondBall(t *testing.T) {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 100)

	r.Split("u1", 0)

	if len(p.Balls) != 2 {
		t.Fatalf("分裂后应有 2 个球, got %d", len(p.Balls))
	}
	// 总质量守恒（分裂不改变总质量）
	if total := p.totalMass(); total < 99.9 || total > 100.1 {
		t.Fatalf("分裂后总质量应约 100, got %v", total)
	}
}

func TestSplitRejectedBelowMinMass(t *testing.T) {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 30) // < MinSplitMass(40)

	r.Split("u1", 0)

	if len(p.Balls) != 1 {
		t.Fatalf("质量不足时不应分裂, got %d balls", len(p.Balls))
	}
}

func TestSplitCooldownBlocksSecond(t *testing.T) {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 200)

	r.Split("u1", 0)
	r.Split("u1", 0) // 立即再次分裂应被冷却拦截

	if len(p.Balls) != 2 {
		t.Fatalf("冷却中第二次分裂应被拦截, got %d balls", len(p.Balls))
	}
}

func TestEjectSpawnsEjectedMass(t *testing.T) {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 100)
	before := p.Balls[0].Mass

	r.Eject("u1", 0)

	if len(r.ejected) != 1 {
		t.Fatalf("吐球后应有 1 个吐出物, got %d", len(r.ejected))
	}
	if p.Balls[0].Mass >= before {
		t.Fatalf("吐球后玩家质量应减少: before=%v after=%v", before, p.Balls[0].Mass)
	}
}

func TestEjectRejectedBelowMinMass(t *testing.T) {
	r := testRoom(t)
	addPlayingHuman(r, "u1", 20) // < MinEjectMass(25)

	r.Eject("u1", 0)

	if len(r.ejected) != 0 {
		t.Fatalf("质量不足时不应吐球, got %d ejected", len(r.ejected))
	}
}

func TestMergeRejoinsBalls(t *testing.T) {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 100)
	// 手工构造两个可合体且重叠的球
	now := int64(1000)
	p.Balls = []*Ball{
		{BallID: "a", X: 1000, Y: 1000, Mass: 50, Radius: Radius(50, r.cfg.RadiusFactor), canMergeAt: now - 1},
		{BallID: "b", X: 1000, Y: 1000, Mass: 50, Radius: Radius(50, r.cfg.RadiusFactor), canMergeAt: now - 1},
	}

	r.mergeBallsLocked(now)

	if len(p.Balls) != 1 {
		t.Fatalf("到期重叠的两球应合体为 1 个, got %d", len(p.Balls))
	}
	if p.Balls[0].Mass != 100 {
		t.Fatalf("合体后质量应为 100, got %v", p.Balls[0].Mass)
	}
}
