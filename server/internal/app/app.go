// Package app 负责组装单体服务：user、match、game、gateway、settlement 各服务
// 共享同一进程内状态和 HTTP mux。
package app

import (
	"fmt"
	"log/slog"
	"net/http"

	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/game"
	"cellular-phagocyte/server/internal/gateway"
	"cellular-phagocyte/server/internal/match"
	"cellular-phagocyte/server/internal/rank"
	"cellular-phagocyte/server/internal/record"
	"cellular-phagocyte/server/internal/redisstore"
	"cellular-phagocyte/server/internal/settlement"
	"cellular-phagocyte/server/internal/user"
)

// stores 持有各域所选的存储实现（内存或 Redis）。
type stores struct {
	user       user.Store
	token      game.TokenStore
	match      match.Store
	settlement settlement.Store
	record     record.Store
	rank       rank.Store
}

// memoryStores 返回全内存存储（默认 / 测试）。
func memoryStores() *stores {
	return &stores{
		user:       user.NewMemoryStore(),
		token:      game.NewMemoryTokenStore(),
		match:      match.NewMemoryStore(),
		settlement: settlement.NewMemoryStore(),
		record:     record.NewMemoryStore(),
		rank:       rank.NewMemoryStore(),
	}
}

// redisStores 用给定 Redis 客户端构造全 Redis 存储。
func redisStores(c *redisstore.Client) *stores {
	return &stores{
		user:       redisstore.NewUserStore(c),
		token:      redisstore.NewTokenStore(c),
		match:      redisstore.NewMatchStore(c),
		settlement: redisstore.NewSettlementStore(c),
		record:     redisstore.NewRecordStore(c),
		rank:       redisstore.NewRankStore(c),
	}
}

// buildStores 按配置选择存储后端。Storage=redis 时连接失败将返回错误。
func buildStores(cfg config.Config, log *slog.Logger) (*stores, error) {
	if cfg.Storage == config.StorageRedis {
		c, err := redisstore.NewClient(cfg.RedisAddr)
		if err != nil {
			return nil, fmt.Errorf("连接 Redis(%s) 失败: %w", cfg.RedisAddr, err)
		}
		log.Info("storage_redis", "addr", cfg.RedisAddr)
		return redisStores(c), nil
	}
	log.Info("storage_memory")
	return memoryStores(), nil
}

// App 保存已组装的各服务以及 HTTP 处理器。
type App struct {
	Cfg     config.Config
	Handler http.Handler

	Users      *user.Service
	Match      *match.Service
	Game       *game.Manager
	Settlement *settlement.Service
	Record     *record.Service
	Rank       *rank.Service
}

// New 根据给定配置构建并组装所有服务。Storage=redis 且连接失败时返回错误。
func New(cfg config.Config, log *slog.Logger) (*App, error) {
	st, err := buildStores(cfg, log)
	if err != nil {
		return nil, err
	}

	users := user.NewService(st.user)
	userH := user.NewHandlers(users)

	mgr := game.NewManager(cfg, users, log, st.token)

	settleSvc := settlement.NewService(users, log, st.settlement)
	mgr.SetSettler(settleSvc)
	settleH := settlement.NewHandlers(settleSvc, userH)

	recordSvc := record.NewService(st.record)
	rankSvc := rank.NewService(st.rank)
	settleSvc.AddSink(recordSvc)
	settleSvc.AddSink(rankSvc)
	recordH := record.NewHandlers(recordSvc, userH)
	rankH := rank.NewHandlers(rankSvc, userH)

	matchSvc := match.NewService(cfg.Match, users, mgr, log, st.match)
	matchH := match.NewHandlers(matchSvc, userH, cfg.Match)

	gw := gateway.New(mgr, log)

	mux := http.NewServeMux()
	userH.Register(mux)
	matchH.Register(mux)
	settleH.Register(mux)
	recordH.Register(mux)
	rankH.Register(mux)
	mux.HandleFunc(cfg.WSPath, gw.Handle)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &App{
		Cfg:        cfg,
		Handler:    withCORS(mux),
		Users:      users,
		Match:      matchSvc,
		Game:       mgr,
		Settlement: settleSvc,
		Record:     recordSvc,
		Rank:       rankSvc,
	}, nil
}

// withCORS 允许浏览器客户端跨域调用 API，并直接响应 OPTIONS 预检请求。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
