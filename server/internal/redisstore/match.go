package redisstore

import (
	"encoding/json"
	"time"

	"cellular-phagocyte/server/internal/match"
)

// 匹配条目有效期（兜底清理，远大于实际匹配时长）。
const matchEntryTTL = 10 * time.Minute

// MatchStore 是 match.Store 的 Redis 实现。
//   match:entry:{matchID} -> JSON 条目（带 TTL）
//   match:user:{userID}   -> matchID（当前活跃条目索引）
//   match:matching        -> SET，保存处于 MATCHING 状态的 matchID
type MatchStore struct{ c *Client }

// NewMatchStore 创建 Redis 匹配队列存储。
func NewMatchStore(c *Client) *MatchStore { return &MatchStore{c: c} }

func (s *MatchStore) Save(e match.Entry) {
	c, cancel := ctx()
	defer cancel()
	raw, _ := json.Marshal(e)
	s.c.rdb.Set(c, "match:entry:"+e.MatchID, raw, matchEntryTTL)
	if e.Status == match.StatusMatching || e.Status == match.StatusMatched {
		s.c.rdb.Set(c, "match:user:"+e.UserID, e.MatchID, matchEntryTTL)
	}
	if e.Status == match.StatusMatching {
		s.c.rdb.SAdd(c, "match:matching", e.MatchID)
	} else {
		s.c.rdb.SRem(c, "match:matching", e.MatchID)
	}
}

func (s *MatchStore) ByMatch(matchID string) (match.Entry, bool) {
	c, cancel := ctx()
	defer cancel()
	raw, err := s.c.rdb.Get(c, "match:entry:"+matchID).Bytes()
	if err != nil {
		return match.Entry{}, false
	}
	var e match.Entry
	if json.Unmarshal(raw, &e) != nil {
		return match.Entry{}, false
	}
	return e, true
}

func (s *MatchStore) ByUser(userID string) (match.Entry, bool) {
	c, cancel := ctx()
	defer cancel()
	mid, err := s.c.rdb.Get(c, "match:user:"+userID).Result()
	if err != nil {
		return match.Entry{}, false
	}
	return s.ByMatch(mid)
}

func (s *MatchStore) DeleteUser(userID string) {
	c, cancel := ctx()
	defer cancel()
	s.c.rdb.Del(c, "match:user:"+userID)
}

func (s *MatchStore) ListMatching() []match.Entry {
	c, cancel := ctx()
	defer cancel()
	ids, err := s.c.rdb.SMembers(c, "match:matching").Result()
	if err != nil {
		return nil
	}
	out := make([]match.Entry, 0, len(ids))
	for _, id := range ids {
		if e, ok := s.ByMatch(id); ok && e.Status == match.StatusMatching {
			out = append(out, e)
		} else {
			// 条目已过期/状态改变，清理索引
			s.c.rdb.SRem(c, "match:matching", id)
		}
	}
	return out
}
