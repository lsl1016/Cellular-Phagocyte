// 命令 server 以单进程内存单体形式运行吞噬细胞游戏服务：
// HTTP API + WebSocket 网关同时监听一个端口。
package main

import (
	"net/http"
	"os"
	"strconv"

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
