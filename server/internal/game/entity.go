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
}

// Food 表示一个静止的食物。
type Food struct {
	ID    string
	X     float64
	Y     float64
	Mass  float64
	Color string
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
