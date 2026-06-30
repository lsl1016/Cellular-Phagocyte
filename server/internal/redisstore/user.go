package redisstore

import (
	"encoding/json"
	"time"

	"cellular-phagocyte/server/internal/user"
)

// 令牌有效期。
const tokenTTL = 24 * time.Hour

// UserStore 是 user.Store 的 Redis 实现。
//   user:seq            -> INCR 自增序列
//   user:{id}           -> JSON 用户
//   user:device:{dev}   -> userID（设备到用户索引）
//   asset:{id}          -> JSON 资产
//   token:{token}       -> userID（带 TTL）
//   biz:{key}           -> 幂等标记（SETNX）
type UserStore struct{ c *Client }

// NewUserStore 创建 Redis 用户存储。
func NewUserStore(c *Client) *UserStore { return &UserStore{c: c} }

// userDTO 显式持有全部字段（user.User 对 DeviceID/CreatedAt 用了 json:"-"）。
type userDTO struct {
	UserID    string `json:"userId"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	UserType  string `json:"userType"`
	Status    string `json:"status"`
	DeviceID  string `json:"deviceId"`
	CreatedAt int64  `json:"createdAt"`
}

func (s *UserStore) NextUserSeq() int64 {
	c, cancel := ctx()
	defer cancel()
	n, _ := s.c.rdb.Incr(c, "user:seq").Result()
	return n
}

func (s *UserStore) GetUser(userID string) (*user.User, bool) {
	c, cancel := ctx()
	defer cancel()
	raw, err := s.c.rdb.Get(c, "user:"+userID).Bytes()
	if err != nil {
		return nil, false
	}
	var d userDTO
	if json.Unmarshal(raw, &d) != nil {
		return nil, false
	}
	return &user.User{
		UserID: d.UserID, Nickname: d.Nickname, Avatar: d.Avatar,
		UserType: d.UserType, Status: d.Status, DeviceID: d.DeviceID, CreatedAt: d.CreatedAt,
	}, true
}

func (s *UserStore) UserIDByDevice(deviceID string) (string, bool) {
	c, cancel := ctx()
	defer cancel()
	uid, err := s.c.rdb.Get(c, "user:device:"+deviceID).Result()
	if err != nil {
		return "", false
	}
	return uid, true
}

func (s *UserStore) SaveUser(u *user.User) {
	c, cancel := ctx()
	defer cancel()
	raw, _ := json.Marshal(userDTO{
		UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar,
		UserType: u.UserType, Status: u.Status, DeviceID: u.DeviceID, CreatedAt: u.CreatedAt,
	})
	s.c.rdb.Set(c, "user:"+u.UserID, raw, 0)
	if u.DeviceID != "" {
		s.c.rdb.Set(c, "user:device:"+u.DeviceID, u.UserID, 0)
	}
}

func (s *UserStore) GetAsset(userID string) (user.Asset, bool) {
	c, cancel := ctx()
	defer cancel()
	raw, err := s.c.rdb.Get(c, "asset:"+userID).Bytes()
	if err != nil {
		return user.Asset{}, false
	}
	var a user.Asset
	if json.Unmarshal(raw, &a) != nil {
		return user.Asset{}, false
	}
	return a, true
}

func (s *UserStore) SaveAsset(a user.Asset) {
	c, cancel := ctx()
	defer cancel()
	raw, _ := json.Marshal(a)
	s.c.rdb.Set(c, "asset:"+a.UserID, raw, 0)
}

func (s *UserStore) PutToken(token, userID string) {
	c, cancel := ctx()
	defer cancel()
	s.c.rdb.Set(c, "token:"+token, userID, tokenTTL)
}

func (s *UserStore) UserIDByToken(token string) (string, bool) {
	c, cancel := ctx()
	defer cancel()
	uid, err := s.c.rdb.Get(c, "token:"+token).Result()
	if err != nil {
		return "", false
	}
	return uid, true
}

func (s *UserStore) MarkBiz(bizKey string) bool {
	c, cancel := ctx()
	defer cancel()
	ok, _ := s.c.rdb.SetNX(c, "biz:"+bizKey, 1, 0).Result()
	return ok
}
