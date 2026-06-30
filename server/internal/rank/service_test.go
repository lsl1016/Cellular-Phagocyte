package rank

import (
	"testing"

	"cellular-phagocyte/server/internal/settlement"
)

func TestOnSettledUpdatesDailyAndBest(t *testing.T) {
	s := NewService()
	// rankPoint = finalScore 1000 + eatPlayer 2*50 + rankBonus(1)=1000 => 2100
	s.OnSettled(settlement.SettledPlayer{UserID: "u1", Nickname: "A", FinalScore: 1000, EatPlayerCount: 2, Rank: 1})

	res, ok := s.Top(TypeDaily, "u1", 1, 50)
	if !ok {
		t.Fatal("daily 榜应存在")
	}
	if len(res.List) != 1 || res.List[0].UserID != "u1" {
		t.Fatalf("榜首应为 u1, got %+v", res.List)
	}
	if res.List[0].Score != 2100 {
		t.Fatalf("daily 分应为 2100, got %d", res.List[0].Score)
	}
	if !res.List[0].Self || res.SelfRank.Rank == nil || *res.SelfRank.Rank != 1 {
		t.Fatalf("自身名次应为 1, got %+v", res.SelfRank)
	}

	best, _ := s.Top(TypeBestScore, "u1", 1, 50)
	if best.List[0].Score != 1000 {
		t.Fatalf("最高分应为 1000, got %d", best.List[0].Score)
	}
}

func TestDailyAccumulatesBestScoreTakesMax(t *testing.T) {
	s := NewService()
	s.OnSettled(settlement.SettledPlayer{UserID: "u1", Nickname: "A", FinalScore: 1000, Rank: 1}) // daily +2000, best 1000
	s.OnSettled(settlement.SettledPlayer{UserID: "u1", Nickname: "A", FinalScore: 500, Rank: 5})  // daily +500+300=800, best 仍 1000

	daily, _ := s.Top(TypeDaily, "u1", 1, 50)
	// 1000+1000 + 500+300 = 2800
	if daily.List[0].Score != 2800 {
		t.Fatalf("daily 应累加为 2800, got %d", daily.List[0].Score)
	}
	best, _ := s.Top(TypeBestScore, "u1", 1, 50)
	if best.List[0].Score != 1000 {
		t.Fatalf("best_score 应取最大 1000, got %d", best.List[0].Score)
	}
}

func TestTopOrdersDescAndMe(t *testing.T) {
	s := NewService()
	s.OnSettled(settlement.SettledPlayer{UserID: "low", Nickname: "L", FinalScore: 100, Rank: 50})
	s.OnSettled(settlement.SettledPlayer{UserID: "high", Nickname: "H", FinalScore: 5000, Rank: 1})

	res, _ := s.Top(TypeDaily, "low", 1, 50)
	if res.List[0].UserID != "high" {
		t.Fatalf("应按分数降序, got %s 居首", res.List[0].UserID)
	}
	me, _ := s.Me(TypeDaily, "low")
	if !me.OnRank || me.Rank == nil || *me.Rank != 2 {
		t.Fatalf("low 应排第 2, got %+v", me)
	}
}

func TestUnknownRankType(t *testing.T) {
	s := NewService()
	if _, ok := s.Top("nope", "", 1, 50); ok {
		t.Fatal("未知榜单类型应返回 false")
	}
}
