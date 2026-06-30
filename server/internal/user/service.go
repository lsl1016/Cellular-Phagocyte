package user

import (
	"errors"
	"fmt"
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

// Store 抽象用户与资产的存储，便于在内存与 Redis 实现间切换。
type Store interface {
	NextUserSeq() int64
	GetUser(userID string) (*User, bool)
	UserIDByDevice(deviceID string) (string, bool)
	SaveUser(u *User)
	GetAsset(userID string) (Asset, bool)
	SaveAsset(a Asset)
	PutToken(token, userID string)
	UserIDByToken(token string) (string, bool)
	// MarkBiz 标记业务幂等键；首次返回 true，重复返回 false。
	MarkBiz(bizKey string) bool
}

// Service 是用户 + 资产服务，业务逻辑在此，存储委托给 Store。
type Service struct {
	store Store
}

// NewService 用给定存储创建用户服务。
func NewService(store Store) *Service {
	return &Service{store: store}
}

// GuestLogin 返回该设备已有的游客或创建一个新游客，并始终签发新的访问令牌。
func (s *Service) GuestLogin(deviceID string) (*User, Asset, string) {
	var u *User
	if deviceID != "" {
		if uid, ok := s.store.UserIDByDevice(deviceID); ok {
			if existing, ok := s.store.GetUser(uid); ok {
				u = existing
			}
		}
	}
	if u == nil {
		seq := s.store.NextUserSeq()
		uid := fmt.Sprintf("%d", 10000+seq)
		u = &User{
			UserID:    uid,
			Nickname:  "吞噬细胞" + idgen.Short(),
			Avatar:    "",
			UserType:  "GUEST",
			Status:    "ACTIVE",
			DeviceID:  deviceID,
			CreatedAt: time.Now().UnixMilli(),
		}
		s.store.SaveUser(u)
		s.store.SaveAsset(Asset{UserID: uid, Coin: 0, Exp: 0, Level: 1})
	}

	token := idgen.Token()
	s.store.PutToken(token, u.UserID)
	asset, _ := s.store.GetAsset(u.UserID)
	return u, asset, token
}

// UserByToken 将访问令牌解析为用户。
func (s *Service) UserByToken(token string) (*User, error) {
	uid, ok := s.store.UserIDByToken(token)
	if !ok {
		return nil, ErrInvalidToken
	}
	u, ok := s.store.GetUser(uid)
	if !ok {
		return nil, ErrInvalidToken
	}
	return u, nil
}

// GetUser 按 id 返回用户。
func (s *Service) GetUser(userID string) (*User, error) {
	u, ok := s.store.GetUser(userID)
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

// GetAsset 返回用户资产。
func (s *Service) GetAsset(userID string) (Asset, error) {
	a, ok := s.store.GetAsset(userID)
	if !ok {
		return Asset{}, ErrUserNotFound
	}
	return a, nil
}

// GrantAssets 按 bizID 幂等地应用金币/经验变更，并按经验表重算等级。
func (s *Service) GrantAssets(userID, bizID string, items []GrantItem) (GrantOutcome, error) {
	a, ok := s.store.GetAsset(userID)
	if !ok {
		return GrantOutcome{}, ErrUserNotFound
	}

	out := GrantOutcome{UserID: userID, OldLevel: a.Level}
	for _, it := range items {
		if !s.store.MarkBiz(bizID + ":" + it.AssetType) {
			continue // 幂等：已经处理过
		}
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

	a.Level = LevelForExp(a.Exp)
	s.store.SaveAsset(a)
	out.NewLevel = a.Level
	out.LevelChanged = a.Level != out.OldLevel
	return out, nil
}
