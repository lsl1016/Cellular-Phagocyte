package match

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/game"
	"cellular-phagocyte/server/internal/user"
)

func testMatchService(t *testing.T) (*Service, Store, *user.User) {
	t.Helper()
	cfg := config.Default()
	cfg.Match.MinStartPlayers = 999
	cfg.Match.MaxWaitSeconds = 3600
	cfg.Match.ScanIntervalMs = int((time.Hour) / time.Millisecond)

	users := user.NewService(user.NewMemoryStore())
	u, _, _ := users.GuestLogin("match-regression-user")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := NewMemoryStore()
	mgr := game.NewManager(cfg, users, log, game.NewMemoryTokenStore())
	return NewService(cfg.Match, users, mgr, log, store), store, u
}

func TestServiceRejectsMatchOwnedByAnotherUser(t *testing.T) {
	svc, store, owner := testMatchService(t)
	entry := Entry{
		MatchID:  "m_owner",
		UserID:   owner.UserID,
		Nickname: owner.Nickname,
		Mode:     "classic",
		JoinedAt: time.Now().UnixMilli(),
		Status:   StatusMatching,
	}
	store.Save(entry)

	if _, ok := svc.Get(entry.MatchID, "another-user"); ok {
		t.Fatal("another user must not read somebody else's match")
	}
	if svc.Cancel(entry.MatchID, "another-user") {
		t.Fatal("another user must not cancel somebody else's match")
	}

	got, ok := svc.Get(entry.MatchID, owner.UserID)
	if !ok || got.MatchID != entry.MatchID {
		t.Fatal("owner should still be able to read the match")
	}
	if !svc.Cancel(entry.MatchID, owner.UserID) {
		t.Fatal("owner should be able to cancel the match")
	}
}

func TestStartReleasesMatchedEntryWhoseRoomIsGone(t *testing.T) {
	svc, store, u := testMatchService(t)
	stale := Entry{
		MatchID:  "m_stale",
		UserID:   u.UserID,
		Nickname: u.Nickname,
		Mode:     "classic",
		JoinedAt: time.Now().Add(-time.Minute).UnixMilli(),
		Status:   StatusMatched,
		RoomID:   "r_already_destroyed",
	}
	store.Save(stale)

	fresh := svc.Start(u, "classic")
	if fresh.MatchID == stale.MatchID {
		t.Fatal("stale MATCHED entry should not block the next game")
	}
	if fresh.Status != StatusMatching {
		t.Fatalf("fresh status = %q, want %q", fresh.Status, StatusMatching)
	}
}
