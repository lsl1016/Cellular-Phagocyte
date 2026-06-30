package settlement

import (
	"sync"

	"cellular-phagocyte/server/internal/protocol"
)

// memoryStore 是 Store 的内存实现（默认 / 测试用）。
type memoryStore struct {
	mu     sync.RWMutex
	byRoom map[string]map[string]protocol.SettlementResultData
	latest map[string]string
}

// NewMemoryStore 创建内存结算存储。
func NewMemoryStore() Store {
	return &memoryStore{
		byRoom: make(map[string]map[string]protocol.SettlementResultData),
		latest: make(map[string]string),
	}
}

func (m *memoryStore) SaveResult(res protocol.SettlementResultData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room := m.byRoom[res.RoomID]
	if room == nil {
		room = make(map[string]protocol.SettlementResultData)
		m.byRoom[res.RoomID] = room
	}
	room[res.UserID] = res
	m.latest[res.UserID] = res.RoomID
}

func (m *memoryStore) Result(roomID, userID string) (protocol.SettlementResultData, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.byRoom[roomID]
	if !ok {
		return protocol.SettlementResultData{}, false
	}
	res, ok := room[userID]
	return res, ok
}

func (m *memoryStore) LatestResult(userID string) (protocol.SettlementResultData, bool) {
	m.mu.RLock()
	roomID, ok := m.latest[userID]
	m.mu.RUnlock()
	if !ok {
		return protocol.SettlementResultData{}, false
	}
	return m.Result(roomID, userID)
}
