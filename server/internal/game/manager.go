// Package game 实现权威游戏服务：房间实例、固定频率的 Tick 循环、移动、
// 碰撞/吞噬、食物、排名以及结算触发。状态全部保存在内存中（MVP 阶段）。
package game

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/idgen"
	"cellular-phagocyte/server/internal/protocol"
	"cellular-phagocyte/server/internal/user"
)

// Conn 是玩家消息写入的网关连接。实现必须保证 Send 非阻塞（缓冲满则丢弃），
// 这样 Tick 循环永远不会因为慢客户端而卡住。
type Conn interface {
	Send(env protocol.Envelope)
	Close()
}

// PlayerInfo 是创建房间时传入的单个玩家数据。
type PlayerInfo struct {
	UserID   string
	Nickname string
	Level    int
}

type tokenEntry struct {
	roomID   string
	userID   string
	expireAt int64
}

// Manager 管理一个游戏服务上的所有房间以及入房凭证注册表。
type Manager struct {
	cfg     config.Config
	users   *user.Service
	log     *slog.Logger
	settler Settler

	mu          sync.RWMutex
	rooms       map[string]*Room
	tokens      map[string]tokenEntry // 入房凭证
	reconTokens map[string]tokenEntry // 重连凭证
}

// Settler 负责发放结算奖励，由结算服务实现。
type Settler interface {
	Settle(r *SettleRequest) []protocol.SettlementResultData
}

// NewManager 创建一个游戏管理器。
func NewManager(cfg config.Config, users *user.Service, log *slog.Logger) *Manager {
	return &Manager{
		cfg:         cfg,
		users:       users,
		log:         log,
		rooms:       make(map[string]*Room),
		tokens:      make(map[string]tokenEntry),
		reconTokens: make(map[string]tokenEntry),
	}
}

// SetSettler 注入结算服务（构造后再注入以避免包循环依赖）。
func (m *Manager) SetSettler(s Settler) { m.settler = s }

// CreateRoom 为匹配到的玩家创建房间，填充机器人、生成食物，并为每个真人玩家
// 签发入房凭证。返回 roomId、各玩家的 token 以及 wsUrl。
func (m *Manager) CreateRoom(matchID, mode string, players []PlayerInfo) (string, map[string]string, string) {
	roomID := "r_" + idgen.Short()
	r := newRoom(roomID, matchID, mode, m.cfg.Game, m, m.log)

	for _, p := range players {
		r.addHuman(p)
	}
	r.addBots(m.cfg.Game.BotFillCount)
	r.spawnInitialFood()

	tokens := make(map[string]string, len(players))
	expireAt := time.Now().UnixMilli() + m.cfg.Match.EnterTokenTTLMs

	m.mu.Lock()
	m.rooms[roomID] = r
	for _, p := range players {
		tok := idgen.Token()
		tokens[p.UserID] = tok
		m.tokens[tok] = tokenEntry{roomID: roomID, userID: p.UserID, expireAt: expireAt}
	}
	m.mu.Unlock()

	wsURL := fmt.Sprintf("ws://%s%s", m.cfg.WSHost, m.cfg.WSPath)
	m.log.Info("room_created", "roomId", roomID, "matchId", matchID, "humans", len(players), "bots", m.cfg.Game.BotFillCount)

	r.startReadyWatchdog()
	return roomID, tokens, wsURL
}

// ValidateEnterToken 校验凭证归属于对应房间/玩家且未过期。
func (m *Manager) ValidateEnterToken(token, roomID, userID string) (*Room, bool) {
	m.mu.RLock()
	te, ok := m.tokens[token]
	r := m.rooms[te.roomID]
	m.mu.RUnlock()
	if !ok || te.roomID != roomID || te.userID != userID {
		return nil, false
	}
	if time.Now().UnixMilli() > te.expireAt {
		return nil, false
	}
	if r == nil {
		return nil, false
	}
	return r, true
}

// issueReconnect 为玩家签发重连凭证。
func (m *Manager) issueReconnect(roomID, userID string) string {
	tok := idgen.Token()
	m.mu.Lock()
	m.reconTokens[tok] = tokenEntry{
		roomID:   roomID,
		userID:   userID,
		expireAt: time.Now().UnixMilli() + m.cfg.Reconnect.TokenTTLMs,
	}
	m.mu.Unlock()
	return tok
}

// ValidateReconnectToken 校验重连凭证归属与有效期。
func (m *Manager) ValidateReconnectToken(token, roomID, userID string) (*Room, bool) {
	m.mu.RLock()
	te, ok := m.reconTokens[token]
	r := m.rooms[te.roomID]
	m.mu.RUnlock()
	if !ok || te.roomID != roomID || te.userID != userID {
		return nil, false
	}
	if time.Now().UnixMilli() > te.expireAt || r == nil {
		return nil, false
	}
	return r, true
}

// Room 按 id 返回房间。
func (m *Manager) Room(roomID string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[roomID]
	return r, ok
}

// removeRoom 删除已结束的房间及其凭证。
func (m *Manager) removeRoom(roomID string) {
	m.mu.Lock()
	delete(m.rooms, roomID)
	for tok, te := range m.tokens {
		if te.roomID == roomID {
			delete(m.tokens, tok)
		}
	}
	for tok, te := range m.reconTokens {
		if te.roomID == roomID {
			delete(m.reconTokens, tok)
		}
	}
	m.mu.Unlock()
	m.log.Info("room_destroy", "roomId", roomID)
}
