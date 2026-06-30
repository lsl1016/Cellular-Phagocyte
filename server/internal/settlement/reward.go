package settlement

import "math"

// PlayerResult 保存驱动奖励计算的单个玩家对局统计。
type PlayerResult struct {
	Rank           int
	EatPlayerCount int
	EatFoodCount   int
	AliveSeconds   int
}

// Reward 是为玩家计算出的金币/经验奖励。
type Reward struct {
	Coin int64
	Exp  int64
}

// RewardConfig 保存奖励公式常量。
type RewardConfig struct {
	BaseCoin      float64
	EatPlayerCoin float64
	EatFoodCoin   float64
	BaseExp       float64
	AliveExpRate  float64
	EatPlayerExp  float64
	MaxCoin       int64
	MaxExp        int64
}

// DefaultRewardConfig 返回推荐的 MVP 奖励常量。
func DefaultRewardConfig() RewardConfig {
	return RewardConfig{
		BaseCoin:      50,
		EatPlayerCoin: 10,
		EatFoodCoin:   0.1,
		BaseExp:       30,
		AliveExpRate:  0.2,
		EatPlayerExp:  5,
		MaxCoin:       1000,
		MaxExp:        800,
	}
}

// rankCoinBonus 将最终排名映射到对应的金币奖励档位。
func rankCoinBonus(rank int) float64 {
	switch {
	case rank == 1:
		return 300
	case rank == 2:
		return 200
	case rank == 3:
		return 150
	case rank >= 4 && rank <= 10:
		return 80
	case rank >= 11 && rank <= 30:
		return 30
	default:
		return 0
	}
}

// rankExpBonus 将最终排名映射到对应的经验奖励档位。
func rankExpBonus(rank int) float64 {
	switch {
	case rank == 1:
		return 200
	case rank == 2:
		return 150
	case rank == 3:
		return 120
	case rank >= 4 && rank <= 10:
		return 60
	case rank >= 11 && rank <= 30:
		return 30
	default:
		return 0
	}
}

// CalcReward 根据玩家结果计算金币和经验奖励，并限制在上限内。
func CalcReward(cfg RewardConfig, p PlayerResult) Reward {
	coin := cfg.BaseCoin +
		rankCoinBonus(p.Rank) +
		float64(p.EatPlayerCount)*cfg.EatPlayerCoin +
		math.Floor(float64(p.EatFoodCount)*cfg.EatFoodCoin)

	exp := cfg.BaseExp +
		rankExpBonus(p.Rank) +
		math.Floor(float64(p.AliveSeconds)*cfg.AliveExpRate) +
		float64(p.EatPlayerCount)*cfg.EatPlayerExp

	return Reward{
		Coin: clamp(int64(coin), cfg.MaxCoin),
		Exp:  clamp(int64(exp), cfg.MaxExp),
	}
}

func clamp(v, max int64) int64 {
	if v > max {
		return max
	}
	if v < 0 {
		return 0
	}
	return v
}
