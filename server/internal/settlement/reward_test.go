package settlement

import "testing"

func TestCalcRewardExample(t *testing.T) {
	cfg := DefaultRewardConfig()
	// 第 1 名，吞噬 7 名玩家，吃掉 236 个食物，存活 300 秒
	r := CalcReward(cfg, PlayerResult{Rank: 1, EatPlayerCount: 7, EatFoodCount: 236, AliveSeconds: 300})

	// coin = base50 + rankBonus300 + 7*10 + floor(236*0.1=23.6)=23 = 443
	if r.Coin != 443 {
		t.Fatalf("coin = %d, want 443", r.Coin)
	}
	// exp = base30 + rankBonus200 + floor(300*0.2=60) + 7*5=35 = 325
	if r.Exp != 325 {
		t.Fatalf("exp = %d, want 325", r.Exp)
	}
}

func TestCalcRewardRankBuckets(t *testing.T) {
	cfg := DefaultRewardConfig()
	// 第 5 名落在 4-10 档：金币奖励 80，经验奖励 60
	r := CalcReward(cfg, PlayerResult{Rank: 5})
	// coin = 50 + 80 = 130 ; exp = 30 + 60 = 90
	if r.Coin != 130 || r.Exp != 90 {
		t.Fatalf("rank5 = %+v, want coin130 exp90", r)
	}
	// 第 50 名落在 31-100 档：奖励为 0
	r = CalcReward(cfg, PlayerResult{Rank: 50})
	if r.Coin != 50 || r.Exp != 30 {
		t.Fatalf("rank50 = %+v, want coin50 exp30", r)
	}
}

func TestCalcRewardCaps(t *testing.T) {
	cfg := DefaultRewardConfig()
	r := CalcReward(cfg, PlayerResult{Rank: 1, EatPlayerCount: 100000, EatFoodCount: 100000, AliveSeconds: 100000})
	if r.Coin != 1000 {
		t.Fatalf("coin cap = %d, want 1000", r.Coin)
	}
	if r.Exp != 800 {
		t.Fatalf("exp cap = %d, want 800", r.Exp)
	}
}
