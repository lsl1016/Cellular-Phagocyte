package user

import "sync"

// memoryStore 是 Store 的内存实现（默认 / 测试用）。
type memoryStore struct {
	mu       sync.RWMutex
	seq      int64
	users    map[string]*User
	assets   map[string]Asset
	tokens   map[string]string
	byDevice map[string]string
	biz      map[string]bool
}

// NewMemoryStore 创建内存用户存储。
func NewMemoryStore() Store {
	return &memoryStore{
		users:    make(map[string]*User),
		assets:   make(map[string]Asset),
		tokens:   make(map[string]string),
		byDevice: make(map[string]string),
		biz:      make(map[string]bool),
	}
}

func (m *memoryStore) NextUserSeq() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	return m.seq
}

func (m *memoryStore) GetUser(userID string) (*User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[userID]
	return u, ok
}

func (m *memoryStore) UserIDByDevice(deviceID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	uid, ok := m.byDevice[deviceID]
	return uid, ok
}

func (m *memoryStore) SaveUser(u *User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.UserID] = u
	if u.DeviceID != "" {
		m.byDevice[u.DeviceID] = u.UserID
	}
}

func (m *memoryStore) GetAsset(userID string) (Asset, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.assets[userID]
	return a, ok
}

func (m *memoryStore) SaveAsset(a Asset) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assets[a.UserID] = a
}

func (m *memoryStore) PutToken(token, userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[token] = userID
}

func (m *memoryStore) UserIDByToken(token string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	uid, ok := m.tokens[token]
	return uid, ok
}

func (m *memoryStore) MarkBiz(bizKey string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.biz[bizKey] {
		return false
	}
	m.biz[bizKey] = true
	return true
}
