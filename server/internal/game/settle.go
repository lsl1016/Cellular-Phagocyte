package game

// SettlePlayerResult 是交给结算系统的单个玩家最终对局结果。
type SettlePlayerResult struct {
	UserID         string
	Nickname       string
	IsBot          bool
	Rank           int
	FinalScore     int64
	MaxMass        int64
	EatPlayerCount int
	EatFoodCount   int
	AliveSeconds   int
	Alive          bool
}

// SettleRequest 是整个房间的结算输入。
type SettleRequest struct {
	RoomID                string
	MatchID               string
	Mode                  string
	StartTime             int64
	EndTime               int64
	BattleDurationSeconds int
	TotalPlayers          int
	Players               []SettlePlayerResult
}
