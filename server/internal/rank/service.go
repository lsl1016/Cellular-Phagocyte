// Package rank 提供全服排行榜：内存维护日榜/周榜（累加 rankPoint）与最高分榜
// （取最大）。结算成功后更新，仅做只读查询。对应设计文档「全服排行榜系统」。
package rank

import (
	"fmt"
	"sort"
	"sync"
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

// Service 是内存排行榜存储。
type Service struct {
	mu     sync.RWMutex
	boards map[string]map[string]int64 // boardKey -> userId -> score
	brief  map[string]string           // userId -> nickname
}

// NewService 创建排行榜服务。
func NewService() *Service {
	return &Service{
		boards: make(map[string]map[string]int64),
		brief:  make(map[string]string),
	}
}

func dailyKey(t time.Time) string  { return "rank:daily:" + t.Format("20060102") }
func weeklyKey(t time.Time) string { y, w := t.ISOWeek(); return fmt.Sprintf("rank:weekly:%04dW%02d", y, w) }

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

	s.mu.Lock()
	defer s.mu.Unlock()
	s.brief[p.UserID] = p.Nickname

	s.addLocked(dailyKey(now), p.UserID, rankPoint)  // 累加
	s.addLocked(weeklyKey(now), p.UserID, rankPoint) // 累加
	s.maxLocked(bestScoreKey, p.UserID, p.FinalScore) // 取最大
}

func (s *Service) addLocked(key, userID string, delta int64) {
	b := s.boards[key]
	if b == nil {
		b = make(map[string]int64)
		s.boards[key] = b
	}
	b[userID] += delta
}

func (s *Service) maxLocked(key, userID string, value int64) {
	b := s.boards[key]
	if b == nil {
		b = make(map[string]int64)
		s.boards[key] = b
	}
	if value > b[userID] {
		b[userID] = value
	}
}

type scored struct {
	userID string
	score  int64
}

// sortedLocked 返回某榜全量按分数降序的列表。
func (s *Service) sortedLocked(key string) []scored {
	b := s.boards[key]
	list := make([]scored, 0, len(b))
	for uid, sc := range b {
		list = append(list, scored{uid, sc})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].score != list[j].score {
			return list[i].score > list[j].score
		}
		return list[i].userID < list[j].userID
	})
	return list
}

// Top 返回某榜分页结果及自身排名。
func (s *Service) Top(rankType, selfUserID string, page, pageSize int) (ListResult, bool) {
	key, ok := boardKeyFor(rankType, time.Now())
	if !ok {
		return ListResult{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sorted := s.sortedLocked(key)
	res := ListResult{
		RankType: rankType, PeriodKey: periodKeyOf(key),
		Page: page, PageSize: pageSize, List: []Entry{},
		RefreshText: refreshText(rankType),
		SelfRank:    SelfRank{OnRank: false},
	}

	for i, sc := range sorted {
		if sc.userID == selfUserID {
			r := i + 1
			res.SelfRank = SelfRank{Rank: &r, Score: sc.score, OnRank: true}
			break
		}
	}

	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start >= len(sorted) {
		return res, true
	}
	end := start + pageSize
	if end > len(sorted) {
		end = len(sorted)
	}
	for i := start; i < end; i++ {
		sc := sorted[i]
		res.List = append(res.List, Entry{
			Rank: i + 1, UserID: sc.userID, Nickname: s.brief[sc.userID],
			Score: sc.score, Self: sc.userID == selfUserID,
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	sorted := s.sortedLocked(key)
	for i, sc := range sorted {
		if sc.userID == userID {
			r := i + 1
			return SelfRank{Rank: &r, Score: sc.score, OnRank: true}, true
		}
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
