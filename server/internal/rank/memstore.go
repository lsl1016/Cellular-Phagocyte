package rank

import (
	"sort"
	"sync"
)

// memoryStore 是 Store 的内存实现（默认 / 测试用）。
type memoryStore struct {
	mu     sync.RWMutex
	boards map[string]map[string]int64 // boardKey -> userId -> score
	brief  map[string]string           // userId -> nickname
}

// NewMemoryStore 创建内存排行榜存储。
func NewMemoryStore() Store {
	return &memoryStore{
		boards: make(map[string]map[string]int64),
		brief:  make(map[string]string),
	}
}

func (m *memoryStore) board(key string) map[string]int64 {
	b := m.boards[key]
	if b == nil {
		b = make(map[string]int64)
		m.boards[key] = b
	}
	return b
}

func (m *memoryStore) IncrBy(key, member string, delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.board(key)[member] += delta
}

func (m *memoryStore) Max(key, member string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b := m.board(key)
	if value > b[member] {
		b[member] = value
	}
}

func (m *memoryStore) SetBrief(userID, nickname string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brief[userID] = nickname
}

func (m *memoryStore) Brief(userID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.brief[userID]
}

// sorted 返回某榜全量按分数降序的列表（分数相同按 userID 升序）。
func (m *memoryStore) sorted(key string) []Scored {
	b := m.boards[key]
	list := make([]Scored, 0, len(b))
	for uid, sc := range b {
		list = append(list, Scored{UserID: uid, Score: sc})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Score != list[j].Score {
			return list[i].Score > list[j].Score
		}
		return list[i].UserID < list[j].UserID
	})
	return list
}

func (m *memoryStore) Range(key string, start, stop int) []Scored {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sorted := m.sorted(key)
	if start < 0 {
		start = 0
	}
	if start >= len(sorted) {
		return []Scored{}
	}
	if stop >= len(sorted) {
		stop = len(sorted) - 1
	}
	if stop < start {
		return []Scored{}
	}
	out := make([]Scored, stop-start+1)
	copy(out, sorted[start:stop+1])
	return out
}

func (m *memoryStore) Card(key string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.boards[key])
}

func (m *memoryStore) RevRank(key, member string) (int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.boards[key][member]; !ok {
		return 0, false
	}
	for i, sc := range m.sorted(key) {
		if sc.UserID == member {
			return i, true
		}
	}
	return 0, false
}

func (m *memoryStore) Score(key, member string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.boards[key][member]
	return v, ok
}
