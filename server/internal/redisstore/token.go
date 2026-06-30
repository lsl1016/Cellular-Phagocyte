package redisstore

import (
	"encoding/json"
	"time"

	"cellular-phagocyte/server/internal/game"
)

// TokenStore 是 game.TokenStore 的 Redis 实现。房间实例仍在内存中，这里仅存凭证。
//   token:enter:{tok} -> JSON 归属（带 TTL）
//   token:recon:{tok} -> JSON 归属（带 TTL）
//   token:room:{roomID} -> SET，保存该房间签发的全部凭证键，便于销毁时清理
type TokenStore struct{ c *Client }

// NewTokenStore 创建 Redis 凭证存储。
func NewTokenStore(c *Client) *TokenStore { return &TokenStore{c: c} }

func ttlFrom(expireAt int64) time.Duration {
	d := time.Until(time.UnixMilli(expireAt))
	if d <= 0 {
		return time.Second
	}
	return d
}

func (s *TokenStore) put(prefix, token string, info game.TokenInfo) {
	c, cancel := ctx()
	defer cancel()
	raw, _ := json.Marshal(info)
	key := prefix + token
	s.c.rdb.Set(c, key, raw, ttlFrom(info.ExpireAt))
	roomKey := "token:room:" + info.RoomID
	s.c.rdb.SAdd(c, roomKey, key)
	s.c.rdb.Expire(c, roomKey, ttlFrom(info.ExpireAt))
}

func (s *TokenStore) get(prefix, token string) (game.TokenInfo, bool) {
	c, cancel := ctx()
	defer cancel()
	raw, err := s.c.rdb.Get(c, prefix+token).Bytes()
	if err != nil {
		return game.TokenInfo{}, false
	}
	var info game.TokenInfo
	if json.Unmarshal(raw, &info) != nil {
		return game.TokenInfo{}, false
	}
	if time.Now().UnixMilli() > info.ExpireAt {
		return game.TokenInfo{}, false
	}
	return info, true
}

func (s *TokenStore) PutEnter(token string, info game.TokenInfo) { s.put("token:enter:", token, info) }
func (s *TokenStore) GetEnter(token string) (game.TokenInfo, bool) { return s.get("token:enter:", token) }
func (s *TokenStore) PutRecon(token string, info game.TokenInfo) { s.put("token:recon:", token, info) }
func (s *TokenStore) GetRecon(token string) (game.TokenInfo, bool) { return s.get("token:recon:", token) }

func (s *TokenStore) DeleteByRoom(roomID string) {
	c, cancel := ctx()
	defer cancel()
	roomKey := "token:room:" + roomID
	keys, err := s.c.rdb.SMembers(c, roomKey).Result()
	if err == nil && len(keys) > 0 {
		s.c.rdb.Del(c, keys...)
	}
	s.c.rdb.Del(c, roomKey)
}
