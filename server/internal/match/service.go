// Package match 实现内存匹配：按模式分组的队列，以及一个在后台通过游戏管理器
// 创建房间的匹配器。
package match

import (
	"log/slog"
	"sort"
	"sync"
	"time"

	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/game"
	"cellular-phagocyte/server/internal/idgen"
	"cellular-phagocyte/server/internal/user"
)

// 匹配状态取值。
const (
	StatusMatching = "MATCHING"
	StatusMatched  = "MATCHED"
	StatusCanceled = "CANCELED"
)

const serverID = "gs_01"

type entry struct {
	matchID  string
	userID   string
	nickname string
	level    int
	mode     string
	joinedAt time.Time
	status   string

	// 匹配成功后填充
	roomID     string
	wsURL      string
	enterToken string
	expireAt   int64
}

// Service 是匹配服务。
type Service struct {
	cfg   config.MatchConfig
	users *user.Service
	mgr   *game.Manager
	log   *slog.Logger

	mu      sync.Mutex
	byMatch map[string]*entry
	byUser  map[string]*entry
}

// NewService 创建匹配服务并启动其匹配循环。
func NewService(cfg config.MatchConfig, users *user.Service, mgr *game.Manager, log *slog.Logger) *Service {
	s := &Service{
		cfg:     cfg,
		users:   users,
		mgr:     mgr,
		log:     log,
		byMatch: make(map[string]*entry),
		byUser:  make(map[string]*entry),
	}
	go s.loop()
	return s
}

// Start 将用户加入匹配队列，返回其（可能已存在的）匹配条目。
func (s *Service) Start(u *user.User, mode string) *entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.byUser[u.UserID]; ok && (e.status == StatusMatching || e.status == StatusMatched) {
		return e
	}

	a, _ := s.users.GetAsset(u.UserID)
	e := &entry{
		matchID:  "m_" + idgen.Short(),
		userID:   u.UserID,
		nickname: u.Nickname,
		level:    a.Level,
		mode:     mode,
		joinedAt: time.Now(),
		status:   StatusMatching,
	}
	s.byMatch[e.matchID] = e
	s.byUser[u.UserID] = e
	s.log.Info("match_start", "matchId", e.matchID, "userId", u.UserID, "mode", mode)
	return e
}

// Cancel 移除一个正在匹配的条目。
func (s *Service) Cancel(matchID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.byMatch[matchID]
	if !ok || e.status != StatusMatching {
		return false
	}
	e.status = StatusCanceled
	delete(s.byUser, e.userID)
	s.log.Info("match_cancel", "matchId", matchID, "userId", e.userID)
	return true
}

// Get 按 id 返回匹配条目。
func (s *Service) Get(matchID string) (*entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.byMatch[matchID]
	return e, ok
}

func (s *Service) loop() {
	interval := time.Duration(s.cfg.ScanIntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	for range time.Tick(interval) {
		s.scan()
	}
}

func (s *Service) scan() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 按模式对等待中的条目分组
	byMode := make(map[string][]*entry)
	for _, e := range s.byMatch {
		if e.status == StatusMatching {
			byMode[e.mode] = append(byMode[e.mode], e)
		}
	}

	for mode, waiting := range byMode {
		sort.Slice(waiting, func(i, j int) bool {
			return waiting[i].joinedAt.Before(waiting[j].joinedAt)
		})
		oldestWait := time.Since(waiting[0].joinedAt).Seconds()
		ready := len(waiting) >= s.cfg.MinStartPlayers ||
			(len(waiting) >= 1 && oldestWait >= float64(s.cfg.MaxWaitSeconds))
		if !ready {
			continue
		}

		group := waiting
		if len(group) > s.cfg.MaxPlayers {
			group = group[:s.cfg.MaxPlayers]
		}
		s.formRoom(mode, group)
	}
}

// formRoom 为一组玩家创建游戏房间，并把每个条目标记为 MATCHED。
func (s *Service) formRoom(mode string, group []*entry) {
	players := make([]game.PlayerInfo, 0, len(group))
	for _, e := range group {
		players = append(players, game.PlayerInfo{
			UserID: e.userID, Nickname: e.nickname, Level: e.level,
		})
	}

	roomID, tokens, wsURL := s.mgr.CreateRoom("m_grp", mode, players)
	expireAt := time.Now().UnixMilli() + s.cfg.EnterTokenTTLMs
	for _, e := range group {
		e.status = StatusMatched
		e.roomID = roomID
		e.wsURL = wsURL
		e.enterToken = tokens[e.userID]
		e.expireAt = expireAt
	}
	s.log.Info("match_success", "roomId", roomID, "mode", mode, "players", len(group))
}

// 供处理器使用的只读访问器（entry 字段是非导出的）。

func (e *entry) MatchID() string    { return e.matchID }
func (e *entry) Status() string     { return e.status }
func (e *entry) RoomID() string     { return e.roomID }
func (e *entry) WsURL() string      { return e.wsURL }
func (e *entry) EnterToken() string { return e.enterToken }
func (e *entry) ServerID() string   { return serverID }
func (e *entry) ExpireAt() int64    { return e.expireAt }

// WaitSeconds 返回该条目已等待的时长。
func (e *entry) WaitSeconds() int { return int(time.Since(e.joinedAt).Seconds()) }
