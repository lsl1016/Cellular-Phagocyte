package game

// 玩家状态取值。
const (
	StatusMatched      = "MATCHED"
	StatusReady        = "READY"
	StatusPlaying      = "PLAYING"
	StatusDead         = "DEAD"
	StatusDisconnected = "DISCONNECTED"
	StatusExited       = "EXITED"
)

// Ball 表示玩家控制的单个球体。
type Ball struct {
	BallID string
	X      float64
	Y      float64
	Mass   float64
	Radius float64

	// 分裂冲量：附加在常规移动之上，按时间衰减
	vx         float64
	vy         float64
	boostUntil int64 // 冲量结束的时间戳（毫秒），0 表示无冲量
	canMergeAt int64 // 该球可与同主球体合体的时间戳（毫秒）
}

// Food 表示一个静止的食物。
type Food struct {
	ID    string
	X     float64
	Y     float64
	Mass  float64
	Color string
}

// EjectedMass 表示玩家吐出的小球，飞行一段后静止，可被任意玩家吞噬。
type EjectedMass struct {
	ID           string
	OwnerID      string
	X            float64
	Y            float64
	Mass         float64
	Radius       float64
	vx           float64
	vy           float64
	moveUntil    int64 // 飞行结束时间戳（毫秒）
	protectUntil int64 // 该时间前原主不可吃回
}

// Player 表示房间中的一个参与者（真人或机器人）。
type Player struct {
	UserID   string
	Nickname string
	IsBot    bool

	Status  string
	Entered bool // 真人是否已完成 ENTER_ROOM
	Ready   bool

	Balls     []*Ball
	Direction float64

	// 下一个 Tick 要应用的待处理输入
	pendingDir   *float64
	lastInputSeq int64

	// 技能冷却
	nextSplitTime   int64
	nextEjectTime   int64
	ejectWindowFrom int64 // 当前 1 秒窗口起点
	ejectInWindow   int

	// 断线重连
	disconnectDeadline int64 // 断线后允许重连的截止时间戳（毫秒），0 表示在线

	// 对局统计
	EatFoodCount   int
	EatPlayerCount int
	MaxMass        float64
	aliveTicks     int64
	dead           bool

	conn Conn
}

// totalMass 汇总玩家所有球体的质量。
func (p *Player) totalMass() float64 {
	var m float64
	for _, b := range p.Balls {
		m += b.Mass
	}
	return m
}

// alive 表示玩家是否仍有球体存活。
func (p *Player) alive() bool {
	return !p.dead && len(p.Balls) > 0
}
