package game

import (
	"sync"
	"time"
)

// TokenInfo 是一条入房/重连凭证的归属信息。
type TokenInfo struct {
	RoomID   string `json:"roomId"`
	UserID   string `json:"userId"`
	ExpireAt int64  `json:"expireAt"` // unix milli
}

// TokenStore 抽象凭证存储（内存 / Redis）。enter 与 recon 两类凭证以前缀区分。
// 注意：游戏房间实例（Room）始终保存在内存中，这里仅存储凭证归属。
type TokenStore interface {
	PutEnter(token string, info TokenInfo)
	GetEnter(token string) (TokenInfo, bool)
	PutRecon(token string, info TokenInfo)
	GetRecon(token string) (TokenInfo, bool)
	DeleteByRoom(roomID string) // 房间销毁时清理其全部凭证
}

// memoryTokenStore 是 TokenStore 的内存实现。
type memoryTokenStore struct {
	mu    sync.RWMutex
	enter map[string]TokenInfo
	recon map[string]TokenInfo
}

// NewMemoryTokenStore 创建内存凭证存储。
func NewMemoryTokenStore() TokenStore {
	return &memoryTokenStore{
		enter: make(map[string]TokenInfo),
		recon: make(map[string]TokenInfo),
	}
}

func (m *memoryTokenStore) PutEnter(token string, info TokenInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enter[token] = info
}

func (m *memoryTokenStore) GetEnter(token string) (TokenInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.enter[token]
	if ok && time.Now().UnixMilli() > info.ExpireAt {
		return TokenInfo{}, false
	}
	return info, ok
}

func (m *memoryTokenStore) PutRecon(token string, info TokenInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recon[token] = info
}

func (m *memoryTokenStore) GetRecon(token string) (TokenInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.recon[token]
	if ok && time.Now().UnixMilli() > info.ExpireAt {
		return TokenInfo{}, false
	}
	return info, ok
}

func (m *memoryTokenStore) DeleteByRoom(roomID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tok, info := range m.enter {
		if info.RoomID == roomID {
			delete(m.enter, tok)
		}
	}
	for tok, info := range m.recon {
		if info.RoomID == roomID {
			delete(m.recon, tok)
		}
	}
}
