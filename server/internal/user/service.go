package user

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"cellular-phagocyte/server/internal/idgen"
)

// User 是一个（游客）账号。
type User struct {
	UserID    string `json:"userId"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	UserType  string `json:"userType"`
	Status    string `json:"status"`
	DeviceID  string `json:"-"`
	CreatedAt int64  `json:"-"`
}

// Asset 保存用户的金币/经验/等级。
type Asset struct {
	UserID string `json:"userId"`
	Coin   int64  `json:"coin"`
	Exp    int64  `json:"exp"`
	Level  int    `json:"level"`
}

// GrantItem 是单条资产变更请求。
type GrantItem struct {
	AssetType string `json:"assetType"` // COIN 或 EXP
	Amount    int64  `json:"amount"`
}

// GrantResult 是单项资产的变更结果。
type GrantResult struct {
	AssetType    string `json:"assetType"`
	BeforeAmount int64  `json:"beforeAmount"`
	ChangeAmount int64  `json:"changeAmount"`
	AfterAmount  int64  `json:"afterAmount"`
}

// GrantOutcome 是 GrantAssets 的完整结果。
type GrantOutcome struct {
	UserID       string        `json:"userId"`
	Results      []GrantResult `json:"results"`
	LevelChanged bool          `json:"levelChanged"`
	OldLevel     int           `json:"oldLevel"`
	NewLevel     int           `json:"newLevel"`
}

var (
	// ErrUserNotFound 在用户 id 未知时返回。
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidToken 在访问令牌未知/过期时返回。
	ErrInvalidToken = errors.New("invalid token")
)

// Service 是内存中的用户 + 资产存储。
type Service struct {
	mu          sync.RWMutex
	users       map[string]*User  // userId -> 用户
	assets      map[string]*Asset // userId -> 资产
	tokens      map[string]string // 访问令牌 -> userId
	byDevice    map[string]string // 设备 deviceId -> userId
	grantedBiz  map[string]bool   // bizId+assetType -> 是否已处理（幂等）
	nextUserNum int64
}

// NewService 创建一个空的用户服务。
func NewService() *Service {
	return &Service{
		users:       make(map[string]*User),
		assets:      make(map[string]*Asset),
		tokens:      make(map[string]string),
		byDevice:    make(map[string]string),
		grantedBiz:  make(map[string]bool),
		nextUserNum: 10000,
	}
}

// GuestLogin 返回该设备已有的游客或创建一个新游客，并始终签发新的访问令牌。
func (s *Service) GuestLogin(deviceID string) (*User, *Asset, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var u *User
	if deviceID != "" {
		if uid, ok := s.byDevice[deviceID]; ok {
			u = s.users[uid]
		}
	}
	if u == nil {
		s.nextUserNum++
		uid := fmt.Sprintf("%d", s.nextUserNum)
		u = &User{
			UserID:    uid,
			Nickname:  "吞噬细胞" + idgen.Short(),
			Avatar:    "",
			UserType:  "GUEST",
			Status:    "ACTIVE",
			DeviceID:  deviceID,
			CreatedAt: time.Now().UnixMilli(),
		}
		s.users[uid] = u
		s.assets[uid] = &Asset{UserID: uid, Coin: 0, Exp: 0, Level: 1}
		if deviceID != "" {
			s.byDevice[deviceID] = uid
		}
	}

	token := idgen.Token()
	s.tokens[token] = u.UserID
	return u, s.assets[u.UserID], token
}

// UserByToken 将访问令牌解析为用户。
func (s *Service) UserByToken(token string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uid, ok := s.tokens[token]
	if !ok {
		return nil, ErrInvalidToken
	}
	return s.users[uid], nil
}

// GetUser 按 id 返回用户。
func (s *Service) GetUser(userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[userID]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

// GetAsset 返回用户资产的副本。
func (s *Service) GetAsset(userID string) (Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.assets[userID]
	if !ok {
		return Asset{}, ErrUserNotFound
	}
	return *a, nil
}

// GrantAssets 按 bizID 幂等地应用金币/经验变更。重复的 bizID+assetType 会被跳过。
// 等级根据总经验重新计算。
func (s *Service) GrantAssets(userID, bizID string, items []GrantItem) (GrantOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.assets[userID]
	if !ok {
		return GrantOutcome{}, ErrUserNotFound
	}

	out := GrantOutcome{UserID: userID, OldLevel: a.Level}
	for _, it := range items {
		key := bizID + ":" + it.AssetType
		if s.grantedBiz[key] {
			continue // 幂等：已经处理过
		}
		s.grantedBiz[key] = true

		res := GrantResult{AssetType: it.AssetType, ChangeAmount: it.Amount}
		switch it.AssetType {
		case "COIN":
			res.BeforeAmount = a.Coin
			a.Coin += it.Amount
			res.AfterAmount = a.Coin
		case "EXP":
			res.BeforeAmount = a.Exp
			a.Exp += it.Amount
			res.AfterAmount = a.Exp
		default:
			continue
		}
		out.Results = append(out.Results, res)
	}

	newLevel := LevelForExp(a.Exp)
	a.Level = newLevel
	out.NewLevel = newLevel
	out.LevelChanged = newLevel != out.OldLevel
	return out, nil
}
