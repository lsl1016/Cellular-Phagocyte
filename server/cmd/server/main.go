// 命令 server 以单进程内存单体形式运行吞噬细胞游戏服务：
// HTTP API + WebSocket 网关同时监听一个端口。
package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"cellular-phagocyte/server/internal/app"
	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/logx"
)

func main() {
	cfg := config.Default()
	if addr := os.Getenv("HTTP_ADDR"); addr != "" {
		cfg.HTTPAddr = addr
	}
	if host := os.Getenv("WS_HOST"); host != "" {
		cfg.WSHost = host
	}
	if s := os.Getenv("STORAGE"); s != "" {
		cfg.Storage = s
	}
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		cfg.RedisAddr = addr
	}

	// 浏览器来源与 WebSocket 输入边界。生产环境建议显式配置 ALLOWED_ORIGINS，
	// 并关闭 loopback 兼容；本地开发默认允许 localhost/127.0.0.1/::1 任意端口。
	if origins := os.Getenv("ALLOWED_ORIGINS"); origins != "" {
		cfg.Security.AllowedOrigins = splitCSV(origins)
	}
	cfg.Security.AllowLoopbackOrigins = envBool("ALLOW_LOOPBACK_ORIGINS", cfg.Security.AllowLoopbackOrigins)
	cfg.Security.WSReadLimitBytes = envInt64("WS_READ_LIMIT_BYTES", cfg.Security.WSReadLimitBytes)

	// 便于本地联调/测试的可选时间参数覆盖。
	cfg.Game.BattleDurationSeconds = envInt("GAME_BATTLE_SECONDS", cfg.Game.BattleDurationSeconds)
	cfg.Game.CountdownSeconds = envInt("GAME_COUNTDOWN_SECONDS", cfg.Game.CountdownSeconds)
	cfg.Game.BotFillCount = envInt("GAME_BOTS", cfg.Game.BotFillCount)
	cfg.Game.PlayerInitialMass = float64(envInt("GAME_INIT_MASS", int(cfg.Game.PlayerInitialMass)))
	cfg.Match.MinStartPlayers = envInt("MATCH_MIN_PLAYERS", cfg.Match.MinStartPlayers)
	cfg.Match.MaxWaitSeconds = envInt("MATCH_MAX_WAIT_SECONDS", cfg.Match.MaxWaitSeconds)

	log := logx.Default()
	a, err := app.New(cfg, log)
	if err != nil {
		log.Error("server_init_failed", "err", err)
		os.Exit(1)
	}

	log.Info("server_start", "addr", cfg.HTTPAddr, "wsPath", cfg.WSPath)
	if err := http.ListenAndServe(cfg.HTTPAddr, a.Handler); err != nil {
		log.Error("server_stopped", "err", err)
		os.Exit(1)
	}
}

// envInt 读取整数环境变量，缺省或非法时返回 def。
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}
