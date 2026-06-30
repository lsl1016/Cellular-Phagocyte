// Package redisstore 提供各域 Store 接口的 Redis 实现。
// 仅迁移可持久化/需跨实例共享的数据；局内实时状态与局内排行榜仍在内存中。
package redisstore

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// 所有操作使用的统一超时上下文时长。
const opTimeout = 3 * time.Second

// Client 包装 go-redis 客户端，供各域 Store 共享。
type Client struct {
	rdb *redis.Client
}

// NewClient 连接到给定地址并返回客户端；连接不可用时返回错误。
func NewClient(addr string) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

// NewClientWithRedis 用已有的 go-redis 客户端构造（供测试注入 miniredis）。
func NewClientWithRedis(rdb *redis.Client) *Client {
	return &Client{rdb: rdb}
}

// Close 关闭底层连接。
func (c *Client) Close() error { return c.rdb.Close() }

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), opTimeout)
}
