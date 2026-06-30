// Package protocol 定义 WebSocket 网关与游戏系统共用的 JSON 协议格式。
// 信封结构：{type, seq, serverTime, traceId, data}。
package protocol

import "encoding/json"

// 客户端 -> 服务端 消息类型。
const (
	TypeEnterRoom = "ENTER_ROOM"
	TypeReady     = "READY"
	TypeMove      = "MOVE"
	TypeSplit     = "SPLIT"
	TypeEject     = "EJECT"
	TypePing      = "PING"
	TypeReconnect = "RECONNECT"
)

// 服务端 -> 客户端 消息类型。
const (
	TypeEnterRoomResult  = "ENTER_ROOM_RESULT"
	TypePlayerReady      = "PLAYER_READY"
	TypeStartCountdown   = "START_COUNTDOWN"
	TypeGameStart        = "GAME_START"
	TypeRoomSnapshot     = "ROOM_SNAPSHOT"
	TypeRankUpdate       = "RANK_UPDATE"
	TypePlayerJoin       = "PLAYER_JOIN"
	TypePlayerLeave      = "PLAYER_LEAVE"
	TypePlayerDead       = "PLAYER_DEAD"
	TypeGameEnd          = "GAME_END"
	TypeSettlementResult = "SETTLEMENT_RESULT"
	TypePong             = "PONG"
	TypeError            = "ERROR"
)

// Envelope 是每条 WebSocket 消息的外层信封。
type Envelope struct {
	Type       string          `json:"type"`
	Seq        int64           `json:"seq,omitempty"`
	ServerTime int64           `json:"serverTime,omitempty"`
	TraceID    string          `json:"traceId,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// EnterRoomData 是 ENTER_ROOM 握手负载（客户端 -> 服务端）。
type EnterRoomData struct {
	RoomID     string `json:"roomId"`
	UserID     string `json:"userId"`
	EnterToken string `json:"enterToken"`
}

// EnterRoomResultData 是 ENTER_ROOM_RESULT 负载（服务端 -> 客户端）。
type EnterRoomResultData struct {
	Success    bool   `json:"success"`
	RoomID     string `json:"roomId,omitempty"`
	Status     string `json:"status,omitempty"`
	ServerTime int64  `json:"serverTime,omitempty"`
	ErrorCode  int    `json:"errorCode,omitempty"`
	Message    string `json:"message,omitempty"`
}

// ReadyData 是 READY 负载（客户端 -> 服务端）。
type ReadyData struct {
	RoomID           string `json:"roomId"`
	UserID           string `json:"userId"`
	ClientLoadCostMs int64  `json:"clientLoadCostMs,omitempty"`
}

// InputData 是基于方向的输入负载，MOVE/SPLIT/EJECT 共用该结构。
type InputData struct {
	Direction  float64 `json:"direction"`
	ClientTime int64   `json:"clientTime,omitempty"`
}

// Ball 是快照中的单个可控球体。
type Ball struct {
	BallID string  `json:"ballId"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Mass   float64 `json:"mass"`
}

// SnapshotPlayer 是 ROOM_SNAPSHOT 中的一个玩家条目。
type SnapshotPlayer struct {
	UserID   string  `json:"userId"`
	Nickname string  `json:"nickname"`
	Status   string  `json:"status"`
	Score    int64   `json:"score"`
	Mass     float64 `json:"mass"`
	Balls    []Ball  `json:"balls"`
}

// SnapshotFood 是 ROOM_SNAPSHOT 中的一个食物条目。
type SnapshotFood struct {
	FoodID string  `json:"foodId"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Mass   float64 `json:"mass"`
	Color  string  `json:"color"`
}

// SnapshotEvent 是一个 Tick 内产生的游戏事件。
type SnapshotEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// RoomSnapshotData 是 ROOM_SNAPSHOT 负载。
type RoomSnapshotData struct {
	RoomID       string           `json:"roomId"`
	SnapshotType string           `json:"snapshotType"`
	TickSeq      int64            `json:"tickSeq"`
	ServerTime   int64            `json:"serverTime"`
	Players      []SnapshotPlayer `json:"players"`
	Foods        []SnapshotFood   `json:"foods"`
	Events       []SnapshotEvent  `json:"events"`
}

// RankEntry 是局内排行榜的一行。
type RankEntry struct {
	Rank     int    `json:"rank"`
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Score    int64  `json:"score"`
}

// SelfRank 是请求方玩家自身的排名。
type SelfRank struct {
	Rank  int   `json:"rank"`
	Score int64 `json:"score"`
}

// RankUpdateData 是 RANK_UPDATE 负载。
type RankUpdateData struct {
	RoomID   string      `json:"roomId"`
	RankTopN []RankEntry `json:"rankTopN"`
	SelfRank *SelfRank   `json:"selfRank,omitempty"`
}

// CountdownData 是 START_COUNTDOWN 负载。
type CountdownData struct {
	RoomID           string `json:"roomId"`
	CountdownSeconds int    `json:"countdownSeconds"`
	ServerStartTime  int64  `json:"serverStartTime"`
}

// GameStartData 是 GAME_START 负载。
type GameStartData struct {
	RoomID                string `json:"roomId"`
	ServerTime            int64  `json:"serverTime"`
	BattleDurationSeconds int    `json:"battleDurationSeconds"`
}

// PlayerReadyData 是 PLAYER_READY 广播负载。
type PlayerReadyData struct {
	RoomID      string `json:"roomId"`
	UserID      string `json:"userId"`
	ReadyCount  int    `json:"readyCount"`
	PlayerCount int    `json:"playerCount"`
}

// GameEndData 是 GAME_END 负载。
type GameEndData struct {
	RoomID  string `json:"roomId"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// SettlementResultData 是 SETTLEMENT_RESULT 负载（每个玩家一条）。
type SettlementResultData struct {
	RoomID         string `json:"roomId"`
	UserID         string `json:"userId"`
	Rank           int    `json:"rank"`
	TotalPlayers   int    `json:"totalPlayers"`
	FinalScore     int64  `json:"finalScore"`
	MaxMass        int64  `json:"maxMass"`
	EatPlayerCount int    `json:"eatPlayerCount"`
	EatFoodCount   int    `json:"eatFoodCount"`
	AliveSeconds   int    `json:"aliveSeconds"`
	CoinReward     int64  `json:"coinReward"`
	ExpReward      int64  `json:"expReward"`
	IsBestScore    bool   `json:"isBestScore"`
	Status         string `json:"status"`
}

// ErrorData 是 ERROR 负载。
type ErrorData struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MustMarshal 序列化 v，出错时返回 nil（用于静态负载）。
func MustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
