package record

import (
	"testing"

	"cellular-phagocyte/server/internal/settlement"
)

func sample(roomID, userID string, rank int, score int64) settlement.SettledPlayer {
	return settlement.SettledPlayer{
		RoomID: roomID, MatchID: "m1", Mode: "classic", UserID: userID, Nickname: "A",
		Rank: rank, TotalPlayers: 4, FinalScore: score, MaxMass: score,
		EatPlayerCount: 2, EatFoodCount: 10, AliveSeconds: 60, Alive: true,
		CoinReward: 50, ExpReward: 30, Status: "SUCCESS", EndTime: 1000,
	}
}

func TestListAndGet(t *testing.T) {
	s := NewService(NewMemoryStore())
	s.OnSettled(sample("r1", "u1", 1, 100))
	s.OnSettled(sample("r2", "u1", 3, 80))
	s.OnSettled(sample("r3", "u2", 1, 200)) // 他人

	total, list := s.List("u1", "", 1, 10)
	if total != 2 || len(list) != 2 {
		t.Fatalf("u1 应有 2 条战绩, got total=%d len=%d", total, len(list))
	}
	// 倒序：最近的 r2 在前
	if list[0].RoomID != "r2" {
		t.Fatalf("应按时间倒序，r2 在前, got %s", list[0].RoomID)
	}
	if list[0].ModeName != "经典模式" {
		t.Fatalf("modeName 应本地化, got %s", list[0].ModeName)
	}

	if _, ok := s.Get("r1", "u1"); !ok {
		t.Fatal("应能查到 r1/u1 详情")
	}
	if _, ok := s.Get("r1", "u2"); ok {
		t.Fatal("不应越权查到他人战绩")
	}
}

func TestSummaryAggregates(t *testing.T) {
	s := NewService(NewMemoryStore())
	s.OnSettled(sample("r1", "u1", 1, 100))
	s.OnSettled(sample("r2", "u1", 3, 80))

	sm := s.Summary("u1")
	if sm.TotalGames != 2 {
		t.Fatalf("总场次应为 2, got %d", sm.TotalGames)
	}
	if sm.FirstPlaceCount != 1 {
		t.Fatalf("第一名次数应为 1, got %d", sm.FirstPlaceCount)
	}
	if sm.BestRank != 1 {
		t.Fatalf("最佳名次应为 1, got %d", sm.BestRank)
	}
	if sm.BestScore != 100 {
		t.Fatalf("最高分应为 100, got %d", sm.BestScore)
	}
	if sm.TotalCoinReward != 100 {
		t.Fatalf("累计金币应为 100, got %d", sm.TotalCoinReward)
	}
}

func TestPagination(t *testing.T) {
	s := NewService(NewMemoryStore())
	for i := 0; i < 25; i++ {
		s.OnSettled(sample("r"+string(rune('a'+i)), "u1", 1, int64(i)))
	}
	total, list := s.List("u1", "", 2, 10)
	if total != 25 || len(list) != 10 {
		t.Fatalf("第二页应有 10 条, total=%d len=%d", total, len(list))
	}
}
