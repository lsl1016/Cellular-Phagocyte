// Package app 负责组装单体服务：user、match、game、gateway、settlement 各服务
// 共享同一进程内状态和 HTTP mux。
package app

import (
	"log/slog"
	"net/http"

	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/game"
	"cellular-phagocyte/server/internal/gateway"
	"cellular-phagocyte/server/internal/match"
	"cellular-phagocyte/server/internal/settlement"
	"cellular-phagocyte/server/internal/user"
)

// App 保存已组装的各服务以及 HTTP 处理器。
type App struct {
	Cfg     config.Config
	Handler http.Handler

	Users      *user.Service
	Match      *match.Service
	Game       *game.Manager
	Settlement *settlement.Service
}

// New 根据给定配置构建并组装所有服务。
func New(cfg config.Config, log *slog.Logger) *App {
	users := user.NewService()
	userH := user.NewHandlers(users)

	mgr := game.NewManager(cfg, users, log)

	settleSvc := settlement.NewService(users, log)
	mgr.SetSettler(settleSvc)
	settleH := settlement.NewHandlers(settleSvc, userH)

	matchSvc := match.NewService(cfg.Match, users, mgr, log)
	matchH := match.NewHandlers(matchSvc, userH, cfg.Match)

	gw := gateway.New(mgr, log)

	mux := http.NewServeMux()
	userH.Register(mux)
	matchH.Register(mux)
	settleH.Register(mux)
	mux.HandleFunc(cfg.WSPath, gw.Handle)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &App{
		Cfg:        cfg,
		Handler:    mux,
		Users:      users,
		Match:      matchSvc,
		Game:       mgr,
		Settlement: settleSvc,
	}
}
