// 命令 server 以单进程内存单体形式运行吞噬细胞游戏服务：
// HTTP API + WebSocket 网关同时监听一个端口。
package main

import (
	"net/http"
	"os"

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

	log := logx.Default()
	a := app.New(cfg, log)

	log.Info("server_start", "addr", cfg.HTTPAddr, "wsPath", cfg.WSPath)
	if err := http.ListenAndServe(cfg.HTTPAddr, a.Handler); err != nil {
		log.Error("server_stopped", "err", err)
		os.Exit(1)
	}
}
