package match

import "sync"

// memoryStore 是 Store 的内存实现（默认 / 测试用）。
type memoryStore struct {
	mu      sync.RWMutex
	byMatch map[string]Entry
	byUser  map[string]string // userID -> matchID（仅指向当前活跃条目）
}

// NewMemoryStore 创建内存匹配队列存储。
func NewMemoryStore() Store {
	return &memoryStore{
		byMatch: make(map[string]Entry),
		byUser:  make(map[string]string),
	}
}

func (m *memoryStore) Save(e Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byMatch[e.MatchID] = e
	if e.Status == StatusMatching || e.Status == StatusMatched {
		m.byUser[e.UserID] = e.MatchID
	}
}

func (m *memoryStore) ByMatch(matchID string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.byMatch[matchID]
	return e, ok
}

func (m *memoryStore) ByUser(userID string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mid, ok := m.byUser[userID]
	if !ok {
		return Entry{}, false
	}
	e, ok := m.byMatch[mid]
	return e, ok
}

func (m *memoryStore) DeleteUser(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.byUser, userID)
}

func (m *memoryStore) ListMatching() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Entry, 0)
	for _, e := range m.byMatch {
		if e.Status == StatusMatching {
			out = append(out, e)
		}
	}
	return out
}
