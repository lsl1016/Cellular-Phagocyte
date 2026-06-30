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

	// 分裂相关
	MinSplitMass         float64 // 可分裂的最小质量
	MaxSplitBalls        int     // 单个玩家最大分身数
	SplitCooldownMs      int64   // 分裂冷却（毫秒）
	SplitMassRatio       float64 // 新球质量 = 源球质量 * 该比例
	SplitBoostSpeed      float64 // 分裂冲量初速度
	SplitBoostDurationMs int64   // 冲量持续时间（毫秒）
	MergeDelaySeconds    int     // 分裂后多少秒可合体

	// 吐球相关
	MinEjectMass     float64 // 可吐球的最小质量
	EjectMass        float64 // 每次吐出/扣除的质量
	EjectIntervalMs  int64   // 两次吐球最小间隔（毫秒）
	MaxEjectPerSec   int     // 每秒最大吐球次数
	EjectSpeed       float64 // 吐出物初速度
	EjectMoveMs      int64   // 吐出物飞行时长（毫秒），之后静止可被吞噬
	EjectGainRatio   float64 // 吞噬吐出物的增重比例
}

// ReconnectConfig 保存断线重连相关常量。
type ReconnectConfig struct {
	WindowSeconds int   // 断线后可重连的窗口（秒）
	TokenTTLMs    int64 // reconnectToken 有效期（毫秒）
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

// 存储后端取值。
const (
	StorageMemory = "memory"
	StorageRedis  = "redis"
)

// Config 是顶层服务配置。
type Config struct {
	HTTPAddr  string
	WSPath    string
	WSHost    string // 在 wsUrl 中告知客户端的 host:port
	Storage   string // memory | redis
	RedisAddr string // Storage=redis 时的连接地址
	Game      GameConfig
	Match     MatchConfig
	Reconnect ReconnectConfig
}

// Default 返回 MVP 默认配置。
func Default() Config {
	return Config{
		HTTPAddr:  ":8080",
		WSPath:    "/ws",
		WSHost:    "localhost:8080",
		Storage:   StorageMemory,
		RedisAddr: "localhost:6379",
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

			MinSplitMass:         40,
			MaxSplitBalls:        8,
			SplitCooldownMs:      1000,
			SplitMassRatio:       0.5,
			SplitBoostSpeed:      600,
			SplitBoostDurationMs: 500,
			MergeDelaySeconds:    10,

			MinEjectMass:    25,
			EjectMass:       5,
			EjectIntervalMs: 150,
			MaxEjectPerSec:  5,
			EjectSpeed:      400,
			EjectMoveMs:     300,
			EjectGainRatio:  1.0,
		},
		Match: MatchConfig{
			MaxPlayers:      100,
			MinStartPlayers: 1,
			MaxWaitSeconds:  3,
			ScanIntervalMs:  500,
			EnterTokenTTLMs: 60000,
		},
		Reconnect: ReconnectConfig{
			WindowSeconds: 30,
			TokenTTLMs:    600000,
		},
	}
}
