// Package record 提供战绩查询：保存每局每玩家的对局结果，结算成功后写入，
// 仅做只读聚合查询。对应设计文档「战绩系统」。存储委托给 Store。
package record

import "cellular-phagocyte/server/internal/settlement"

// Entry 是单局单玩家的战绩记录。
type Entry struct {
	RoomID          string `json:"roomId"`
	MatchID         string `json:"matchId"`
	Mode            string `json:"mode"`
	ModeName        string `json:"modeName"`
	Rank            int    `json:"rank"`
	TotalPlayers    int    `json:"totalPlayers"`
	FinalScore      int64  `json:"finalScore"`
	MaxMass         int64  `json:"maxMass"`
	EatPlayerCount  int    `json:"eatPlayerCount"`
	EatFoodCount    int    `json:"eatFoodCount"`
	AliveSeconds    int    `json:"aliveSeconds"`
	Alive           bool   `json:"alive"`
	CoinReward      int64  `json:"coinReward"`
	ExpReward       int64  `json:"expReward"`
	Status          string `json:"status"`
	StartTime       int64  `json:"startTime"`
	EndTime         int64  `json:"endTime"`
	DurationSeconds int    `json:"durationSeconds"`
}

// Summary 是个人战绩统计概览。
type Summary struct {
	TotalGames          int   `json:"totalGames"`
	FirstPlaceCount     int   `json:"firstPlaceCount"`
	Top3Count           int   `json:"top3Count"`
	Top10Count          int   `json:"top10Count"`
	BestRank            int   `json:"bestRank"`
	BestScore           int64 `json:"bestScore"`
	MaxMass             int64 `json:"maxMass"`
	MaxEatPlayerCount   int   `json:"maxEatPlayerCount"`
	TotalEatPlayerCount int   `json:"totalEatPlayerCount"`
	TotalEatFoodCount   int   `json:"totalEatFoodCount"`
	TotalCoinReward     int64 `json:"totalCoinReward"`
	TotalExpReward      int64 `json:"totalExpReward"`
}

func modeName(mode string) string {
	if mode == "classic" {
		return "经典模式"
	}
	return mode
}

// Store 抽象战绩存储（内存 / Redis）。ListByUser 返回按时间升序的全部记录。
type Store interface {
	Append(userID string, e Entry)
	ListByUser(userID string) []Entry
	Get(roomID, userID string) (Entry, bool)
}

// Service 是战绩服务。
type Service struct {
	store Store
}

// NewService 用给定存储创建战绩服务。
func NewService(store Store) *Service {
	return &Service{store: store}
}

// OnSettled 实现 settlement.Sink：结算成功后写入一条战绩。
func (s *Service) OnSettled(p settlement.SettledPlayer) {
	s.store.Append(p.UserID, Entry{
		RoomID: p.RoomID, MatchID: p.MatchID, Mode: p.Mode, ModeName: modeName(p.Mode),
		Rank: p.Rank, TotalPlayers: p.TotalPlayers,
		FinalScore: p.FinalScore, MaxMass: p.MaxMass,
		EatPlayerCount: p.EatPlayerCount, EatFoodCount: p.EatFoodCount,
		AliveSeconds: p.AliveSeconds, Alive: p.Alive,
		CoinReward: p.CoinReward, ExpReward: p.ExpReward, Status: p.Status,
		StartTime: p.StartTime, EndTime: p.EndTime, DurationSeconds: p.DurationSeconds,
	})
}

// List 返回某用户的战绩分页列表（按 endTime 倒序），可按 mode 过滤。
func (s *Service) List(userID, mode string, page, pageSize int) (total int, list []Entry) {
	all := s.store.ListByUser(userID)
	filtered := make([]Entry, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- { // 倒序
		if mode == "" || all[i].Mode == mode {
			filtered = append(filtered, all[i])
		}
	}
	total = len(filtered)

	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start >= total {
		return total, []Entry{}
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return total, filtered[start:end]
}

// Get 返回某用户某局战绩详情。
func (s *Service) Get(roomID, userID string) (Entry, bool) {
	return s.store.Get(roomID, userID)
}

// Summary 返回某用户的统计概览（实时聚合）。
func (s *Service) Summary(userID string) Summary {
	var sm Summary
	for _, e := range s.store.ListByUser(userID) {
		sm.TotalGames++
		if e.Rank == 1 {
			sm.FirstPlaceCount++
		}
		if e.Rank <= 3 {
			sm.Top3Count++
		}
		if e.Rank <= 10 {
			sm.Top10Count++
		}
		if sm.BestRank == 0 || e.Rank < sm.BestRank {
			sm.BestRank = e.Rank
		}
		if e.FinalScore > sm.BestScore {
			sm.BestScore = e.FinalScore
		}
		if e.MaxMass > sm.MaxMass {
			sm.MaxMass = e.MaxMass
		}
		if e.EatPlayerCount > sm.MaxEatPlayerCount {
			sm.MaxEatPlayerCount = e.EatPlayerCount
		}
		sm.TotalEatPlayerCount += e.EatPlayerCount
		sm.TotalEatFoodCount += e.EatFoodCount
		sm.TotalCoinReward += e.CoinReward
		sm.TotalExpReward += e.ExpReward
	}
	return sm
}

// Latest 返回某用户最近一局战绩。
func (s *Service) Latest(userID string) (Entry, bool) {
	all := s.store.ListByUser(userID)
	if len(all) == 0 {
		return Entry{}, false
	}
	return all[len(all)-1], true
}
