// Package config 保存单体服务的运行时配置。
package config

// GameConfig 保存单个房间的玩法常量。默认值遵循设计文档，但对局时长被缩短
// 以便本地 MVP 快速测试。
type GameConfig struct {
	TickRate              int     // 逻辑 Tick 每秒次数
	SnapshotRate          int     // 快照广播每秒次数
	RankUpdateRate        int     // 排行榜推送每秒次数
	CountdownSeconds      int     // 开局前倒计时时长
	BattleDurationSeconds int     // 单局对战时长
	MapWidth              float64 // 地图宽度
	MapHeight             float64 // 地图高度
	InitialFoodCount      int     // 开局生成的食物数量
	MaxFoodCount          int     // 食物数量上限
	PlayerInitialMass     float64 // 玩家初始质量
	BaseSpeed             float64 // 初始质量时的基础移动速度（单位/秒）
	RadiusFactor          float64 // radius = sqrt(mass) * RadiusFactor
	EatMassRatio          float64 // 攻击方质量需超过 目标*该比例 才能吞噬
	EatDepthFactor        float64 // 玩家吞噬的重叠深度系数
	PlayerEatMassGain     float64 // 获得质量 = 目标质量 * 该系数
	FoodMass              float64 // 单个食物的质量
	BotFillCount          int     // 填充房间以便单人可玩的机器人数量
	BotInitialMass        float64 // 机器人初始质量
}

// MatchConfig 保存匹配相关常量。为便于单人 MVP 测试，开局门槛很低、等待很短，
// 使单个真人玩家也能快速开始。
type MatchConfig struct {
	MaxPlayers      int
	MinStartPlayers int
	MaxWaitSeconds  int
	ScanIntervalMs  int
	EnterTokenTTLMs int64
}

// Config 是顶层服务配置。
type Config struct {
	HTTPAddr string
	WSPath   string
	WSHost   string // 在 wsUrl 中告知客户端的 host:port
	Game     GameConfig
	Match    MatchConfig
}

// Default 返回 MVP 默认配置。
func Default() Config {
	return Config{
		HTTPAddr: ":8080",
		WSPath:   "/ws",
		WSHost:   "localhost:8080",
		Game: GameConfig{
			TickRate:              20,
			SnapshotRate:          10,
			RankUpdateRate:        1,
			CountdownSeconds:      3,
			BattleDurationSeconds: 60,
			MapWidth:              4000,
			MapHeight:             4000,
			InitialFoodCount:      400,
			MaxFoodCount:          500,
			PlayerInitialMass:     20,
			BaseSpeed:             280,
			RadiusFactor:          2.5,
			EatMassRatio:          1.2,
			EatDepthFactor:        0.4,
			PlayerEatMassGain:     0.8,
			FoodMass:              1,
			BotFillCount:          8,
			BotInitialMass:        20,
		},
		Match: MatchConfig{
			MaxPlayers:      100,
			MinStartPlayers: 1,
			MaxWaitSeconds:  3,
			ScanIntervalMs:  500,
			EnterTokenTTLMs: 60000,
		},
	}
}
