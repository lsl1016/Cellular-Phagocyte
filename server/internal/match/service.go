// Package match 实现匹配：按模式分组的队列，以及一个在后台通过游戏管理器
// 创建房间的匹配器。队列条目存储委托给 Store（内存 / Redis）。
package match

import (
	"log/slog"
	"sort"
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

// Entry 是一条匹配队列条目（含匹配成功后回填的房间信息）。
type Entry struct {
	MatchID  string `json:"matchId"`
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Level    int    `json:"level"`
	Mode     string `json:"mode"`
	JoinedAt int64  `json:"joinedAt"` // unix milli
	Status   string `json:"status"`

	// 匹配成功后填充
	RoomID     string `json:"roomId"`
	WsURL      string `json:"wsUrl"`
	EnterToken string `json:"enterToken"`
	ExpireAt   int64  `json:"expireAt"`
}

// ServerID 返回所在游戏服 id。
func (e Entry) ServerID() string { return serverID }

// WaitSeconds 返回该条目已等待的时长（秒）。
func (e Entry) WaitSeconds() int {
	return int(time.Since(time.UnixMilli(e.JoinedAt)).Seconds())
}

// Store 抽象匹配队列存储（内存 / Redis）。
type Store interface {
	Save(e Entry)
	ByMatch(matchID string) (Entry, bool)
	ByUser(userID string) (Entry, bool)
	DeleteUser(userID string)
	ListMatching() []Entry
}

// Service 是匹配服务。
type Service struct {
	cfg   config.MatchConfig
	users *user.Service
	mgr   *game.Manager
	log   *slog.Logger
	store Store
}

// NewService 用给定存储创建匹配服务并启动其匹配循环。
func NewService(cfg config.MatchConfig, users *user.Service, mgr *game.Manager, log *slog.Logger, store Store) *Service {
	s := &Service{
		cfg:   cfg,
		users: users,
		mgr:   mgr,
		log:   log,
		store: store,
	}
	go s.loop()
	return s
}

// Start 将用户加入匹配队列，返回其（可能已存在的）匹配条目。
func (s *Service) Start(u *user.User, mode string) Entry {
	if e, ok := s.store.ByUser(u.UserID); ok && (e.Status == StatusMatching || e.Status == StatusMatched) {
		return e
	}

	a, _ := s.users.GetAsset(u.UserID)
	e := Entry{
		MatchID:  "m_" + idgen.Short(),
		UserID:   u.UserID,
		Nickname: u.Nickname,
		Level:    a.Level,
		Mode:     mode,
		JoinedAt: time.Now().UnixMilli(),
		Status:   StatusMatching,
	}
	s.store.Save(e)
	s.log.Info("match_start", "matchId", e.MatchID, "userId", u.UserID, "mode", mode)
	return e
}

// Cancel 移除一个正在匹配的条目。
func (s *Service) Cancel(matchID string) bool {
	e, ok := s.store.ByMatch(matchID)
	if !ok || e.Status != StatusMatching {
		return false
	}
	e.Status = StatusCanceled
	s.store.Save(e)
	s.store.DeleteUser(e.UserID)
	s.log.Info("match_cancel", "matchId", matchID, "userId", e.UserID)
	return true
}

// Get 按 id 返回匹配条目。
func (s *Service) Get(matchID string) (Entry, bool) {
	return s.store.ByMatch(matchID)
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
	// 按模式对等待中的条目分组
	byMode := make(map[string][]Entry)
	for _, e := range s.store.ListMatching() {
		byMode[e.Mode] = append(byMode[e.Mode], e)
	}

	for mode, waiting := range byMode {
		sort.Slice(waiting, func(i, j int) bool {
			return waiting[i].JoinedAt < waiting[j].JoinedAt
		})
		oldestWait := time.Since(time.UnixMilli(waiting[0].JoinedAt)).Seconds()
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
func (s *Service) formRoom(mode string, group []Entry) {
	players := make([]game.PlayerInfo, 0, len(group))
	for _, e := range group {
		players = append(players, game.PlayerInfo{
			UserID: e.UserID, Nickname: e.Nickname, Level: e.Level,
		})
	}

	roomID, tokens, wsURL := s.mgr.CreateRoom("m_grp", mode, players)
	expireAt := time.Now().UnixMilli() + s.cfg.EnterTokenTTLMs
	for _, e := range group {
		e.Status = StatusMatched
		e.RoomID = roomID
		e.WsURL = wsURL
		e.EnterToken = tokens[e.UserID]
		e.ExpireAt = expireAt
		s.store.Save(e)
	}
	s.log.Info("match_success", "roomId", roomID, "mode", mode, "players", len(group))
}
