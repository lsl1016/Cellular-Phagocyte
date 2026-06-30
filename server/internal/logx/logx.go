// Package logx 提供一个基于 slog 的轻量结构化（JSON）日志器。
package logx

import (
	"log/slog"
	"os"
)

// New 返回一个写入 stdout 的 JSON slog.Logger，使用给定日志级别。
func New(level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(h)
}

// Default 返回 INFO 级别的 JSON 日志器。
func Default() *slog.Logger {
	return New(slog.LevelInfo)
}
