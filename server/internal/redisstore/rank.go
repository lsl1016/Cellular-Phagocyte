package redisstore

import (
	"strings"
	"time"

	"cellular-phagocyte/server/internal/rank"
	"github.com/redis/go-redis/v9"
)

// RankStore 是 rank.Store 的 Redis 实现，直接映射到 ZSET。
//   rank:daily:{date}  ZSET（累加，TTL 2 天）
//   rank:weekly:{week} ZSET（累加，TTL 8 天）
//   rank:best_score    ZSET（取最大）
//   rank:brief         HASH userID -> nickname
// 说明：ZSET 同分时按 member 字典序（ZREVRANGE 为逆序）排序，与内存实现的
// 同分按 userID 升序略有差异，但分数通常不同，不影响榜单结果。
type RankStore struct{ c *Client }

// NewRankStore 创建 Redis 排行榜存储。
func NewRankStore(c *Client) *RankStore { return &RankStore{c: c} }

func boardTTL(key string) time.Duration {
	switch {
	case strings.HasPrefix(key, "rank:daily:"):
		return 48 * time.Hour
	case strings.HasPrefix(key, "rank:weekly:"):
		return 8 * 24 * time.Hour
	default:
		return 0
	}
}

func (s *RankStore) IncrBy(key, member string, delta int64) {
	c, cancel := ctx()
	defer cancel()
	s.c.rdb.ZIncrBy(c, key, float64(delta), member)
	if ttl := boardTTL(key); ttl > 0 {
		s.c.rdb.Expire(c, key, ttl)
	}
}

func (s *RankStore) Max(key, member string, value int64) {
	c, cancel := ctx()
	defer cancel()
	cur, err := s.c.rdb.ZScore(c, key, member).Result()
	if err == nil && int64(cur) >= value {
		return
	}
	s.c.rdb.ZAdd(c, key, redis.Z{Score: float64(value), Member: member})
	if ttl := boardTTL(key); ttl > 0 {
		s.c.rdb.Expire(c, key, ttl)
	}
}

func (s *RankStore) SetBrief(userID, nickname string) {
	c, cancel := ctx()
	defer cancel()
	s.c.rdb.HSet(c, "rank:brief", userID, nickname)
}

func (s *RankStore) Brief(userID string) string {
	c, cancel := ctx()
	defer cancel()
	v, _ := s.c.rdb.HGet(c, "rank:brief", userID).Result()
	return v
}

func (s *RankStore) Range(key string, start, stop int) []rank.Scored {
	c, cancel := ctx()
	defer cancel()
	zs, err := s.c.rdb.ZRevRangeWithScores(c, key, int64(start), int64(stop)).Result()
	if err != nil {
		return nil
	}
	out := make([]rank.Scored, 0, len(zs))
	for _, z := range zs {
		member, _ := z.Member.(string)
		out = append(out, rank.Scored{UserID: member, Score: int64(z.Score)})
	}
	return out
}

func (s *RankStore) Card(key string) int {
	c, cancel := ctx()
	defer cancel()
	n, _ := s.c.rdb.ZCard(c, key).Result()
	return int(n)
}

func (s *RankStore) RevRank(key, member string) (int, bool) {
	c, cancel := ctx()
	defer cancel()
	r, err := s.c.rdb.ZRevRank(c, key, member).Result()
	if err != nil {
		return 0, false
	}
	return int(r), true
}

func (s *RankStore) Score(key, member string) (int64, bool) {
	c, cancel := ctx()
	defer cancel()
	sc, err := s.c.rdb.ZScore(c, key, member).Result()
	if err != nil {
		return 0, false
	}
	return int64(sc), true
}
