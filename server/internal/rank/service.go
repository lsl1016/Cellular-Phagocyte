// Package rank 提供全服排行榜：维护日榜/周榜（累加 rankPoint）与最高分榜
// （取最大）。结算成功后更新，仅做只读查询。对应设计文档「全服排行榜系统」。
// 存储委托给 Store（内存 / Redis ZSET）。
package rank

import (
	"fmt"
	"time"

	"cellular-phagocyte/server/internal/settlement"
)

// 榜单类型。
const (
	TypeDaily     = "daily"
	TypeWeekly    = "weekly"
	TypeBestScore = "best_score"
)

// Entry 是榜单中的一行。
type Entry struct {
	Rank     int    `json:"rank"`
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Score    int64  `json:"score"`
	Self     bool   `json:"self"`
}

// SelfRank 是请求者自身排名；未上榜时 Rank 为 null。
type SelfRank struct {
	Rank   *int  `json:"rank"`
	Score  int64 `json:"score"`
	OnRank bool  `json:"onRank"`
}

// ListResult 是榜单查询结果。
type ListResult struct {
	RankType    string   `json:"rankType"`
	PeriodKey   string   `json:"periodKey"`
	Page        int      `json:"page"`
	PageSize    int      `json:"pageSize"`
	List        []Entry  `json:"list"`
	SelfRank    SelfRank `json:"selfRank"`
	RefreshText string   `json:"refreshText"`
}

// Scored 是榜单成员及其分数。
type Scored struct {
	UserID string
	Score  int64
}

// Store 抽象排行榜存储（内存 / Redis ZSET）。语义与 Redis ZSET 对齐：
// 排名均为降序（分数高者在前），名次从 0 开始。
type Store interface {
	IncrBy(key, member string, delta int64) // ZINCRBY：日榜/周榜累加
	Max(key, member string, value int64)     // best_score：仅当更大时更新
	SetBrief(userID, nickname string)         // 记录昵称
	Brief(userID string) string               // 取昵称
	Range(key string, start, stop int) []Scored // ZREVRANGE：降序，含两端，0 起
	Card(key string) int                          // ZCARD：成员总数
	RevRank(key, member string) (int, bool)        // ZREVRANK：降序名次，0 起
	Score(key, member string) (int64, bool)        // ZSCORE
}

// Service 是排行榜服务。
type Service struct {
	store Store
}

// NewService 用给定存储创建排行榜服务。
func NewService(store Store) *Service {
	return &Service{store: store}
}

func dailyKey(t time.Time) string { return "rank:daily:" + t.Format("20060102") }
func weeklyKey(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("rank:weekly:%04dW%02d", y, w)
}

const bestScoreKey = "rank:best_score"

func boardKeyFor(rankType string, t time.Time) (string, bool) {
	switch rankType {
	case TypeDaily:
		return dailyKey(t), true
	case TypeWeekly:
		return weeklyKey(t), true
	case TypeBestScore:
		return bestScoreKey, true
	default:
		return "", false
	}
}

func refreshText(rankType string) string {
	switch rankType {
	case TypeDaily:
		return "今日榜"
	case TypeWeekly:
		return "本周榜"
	case TypeBestScore:
		return "最高分榜"
	default:
		return ""
	}
}

// rankBonus 按名次返回排行榜加分。
func rankBonus(rank int) int64 {
	switch {
	case rank == 1:
		return 1000
	case rank == 2:
		return 700
	case rank == 3:
		return 500
	case rank >= 4 && rank <= 10:
		return 300
	case rank >= 11 && rank <= 30:
		return 100
	default:
		return 0
	}
}

// OnSettled 实现 settlement.Sink：结算成功后更新各榜。
func (s *Service) OnSettled(p settlement.SettledPlayer) {
	rankPoint := p.FinalScore + int64(p.EatPlayerCount)*50 + rankBonus(p.Rank)
	now := time.Now()

	s.store.SetBrief(p.UserID, p.Nickname)
	s.store.IncrBy(dailyKey(now), p.UserID, rankPoint)  // 累加
	s.store.IncrBy(weeklyKey(now), p.UserID, rankPoint) // 累加
	s.store.Max(bestScoreKey, p.UserID, p.FinalScore)   // 取最大
}

// Top 返回某榜分页结果及自身排名。
func (s *Service) Top(rankType, selfUserID string, page, pageSize int) (ListResult, bool) {
	key, ok := boardKeyFor(rankType, time.Now())
	if !ok {
		return ListResult{}, false
	}

	res := ListResult{
		RankType: rankType, PeriodKey: periodKeyOf(key),
		Page: page, PageSize: pageSize, List: []Entry{},
		RefreshText: refreshText(rankType),
		SelfRank:    SelfRank{OnRank: false},
	}

	if r, ok := s.store.RevRank(key, selfUserID); ok {
		sc, _ := s.store.Score(key, selfUserID)
		rank := r + 1
		res.SelfRank = SelfRank{Rank: &rank, Score: sc, OnRank: true}
	}

	total := s.store.Card(key)
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start >= total {
		return res, true
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	for i, sc := range s.store.Range(key, start, end-1) {
		rank := start + i + 1
		res.List = append(res.List, Entry{
			Rank: rank, UserID: sc.UserID, Nickname: s.store.Brief(sc.UserID),
			Score: sc.Score, Self: sc.UserID == selfUserID,
		})
	}
	return res, true
}

// Me 返回自身在某榜的排名。
func (s *Service) Me(rankType, userID string) (SelfRank, bool) {
	key, ok := boardKeyFor(rankType, time.Now())
	if !ok {
		return SelfRank{}, false
	}
	if r, ok := s.store.RevRank(key, userID); ok {
		sc, _ := s.store.Score(key, userID)
		rank := r + 1
		return SelfRank{Rank: &rank, Score: sc, OnRank: true}, true
	}
	return SelfRank{OnRank: false}, true
}

func periodKeyOf(boardKey string) string {
	// boardKey 形如 rank:daily:20260628 / rank:weekly:2026W26 / rank:best_score
	for i := len(boardKey) - 1; i >= 0; i-- {
		if boardKey[i] == ':' {
			return boardKey[i+1:]
		}
	}
	return boardKey
}
