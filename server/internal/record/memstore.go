package record

import "sync"

// memoryStore 是 Store 的内存实现（默认 / 测试用）。
type memoryStore struct {
	mu     sync.RWMutex
	byUser map[string][]Entry
	byRoom map[string]map[string]Entry
}

// NewMemoryStore 创建内存战绩存储。
func NewMemoryStore() Store {
	return &memoryStore{
		byUser: make(map[string][]Entry),
		byRoom: make(map[string]map[string]Entry),
	}
}

func (m *memoryStore) Append(userID string, e Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byUser[userID] = append(m.byUser[userID], e)
	room := m.byRoom[e.RoomID]
	if room == nil {
		room = make(map[string]Entry)
		m.byRoom[e.RoomID] = room
	}
	room[userID] = e
}

func (m *memoryStore) ListByUser(userID string) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.byUser[userID]
	out := make([]Entry, len(src))
	copy(out, src)
	return out
}

func (m *memoryStore) Get(roomID, userID string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.byRoom[roomID]
	if !ok {
		return Entry{}, false
	}
	e, ok := room[userID]
	return e, ok
}
