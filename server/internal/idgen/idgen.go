// Package idgen 为用户、房间、凭证等生成唯一标识。
package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
)

var counter uint64

// Token 返回 32 个十六进制字符（16 字节）的随机凭证。
func Token() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Short 返回一个较短的随机十六进制字符串（8 个字符）。
func Short() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Seq 返回进程内单调递增的序列号。
func Seq() uint64 {
	return atomic.AddUint64(&counter, 1)
}
