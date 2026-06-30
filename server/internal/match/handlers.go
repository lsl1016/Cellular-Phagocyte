package match

import (
	"net/http"
	"time"

	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/httpx"
	"cellular-phagocyte/server/internal/user"
)

// Handlers 暴露匹配 HTTP API。
type Handlers struct {
	svc  *Service
	auth *user.Handlers
	cfg  config.MatchConfig
}

// NewHandlers 关联匹配 HTTP 处理器。
func NewHandlers(svc *Service, auth *user.Handlers, cfg config.MatchConfig) *Handlers {
	return &Handlers{svc: svc, auth: auth, cfg: cfg}
}

// Register 挂载匹配路由。
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/match/start", h.start)
	mux.HandleFunc("POST /api/match/cancel", h.cancel)
	mux.HandleFunc("GET /api/match/status", h.status)
	mux.HandleFunc("GET /api/match/config", h.config)
}

type startReq struct {
	Mode          string `json:"mode"`
	ClientVersion string `json:"clientVersion"`
	Region        string `json:"region"`
}

func (h *Handlers) start(w http.ResponseWriter, r *http.Request) {
	u, ok := h.auth.AuthUser(w, r)
	if !ok {
		return
	}
	var req startReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Mode == "" {
		req.Mode = "classic"
	}
	e := h.svc.Start(u, req.Mode)
	httpx.WriteOK(w, map[string]any{
		"matchId":              e.MatchID,
		"status":               e.Status,
		"estimatedWaitSeconds": h.cfg.MaxWaitSeconds,
		"serverTime":           time.Now().UnixMilli(),
	})
}

type cancelReq struct {
	MatchID string `json:"matchId"`
}

func (h *Handlers) cancel(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.auth.AuthUser(w, r); !ok {
		return
	}
	var req cancelReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !h.svc.Cancel(req.MatchID) {
		httpx.WriteErr(w, http.StatusBadRequest, 20003, "匹配已取消或不存在")
		return
	}
	httpx.WriteOK(w, map[string]any{"status": StatusCanceled})
}

func (h *Handlers) status(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.auth.AuthUser(w, r); !ok {
		return
	}
	matchID := r.URL.Query().Get("matchId")
	e, ok := h.svc.Get(matchID)
	if !ok {
		httpx.WriteErr(w, http.StatusNotFound, 20005, "匹配不存在")
		return
	}
	if e.Status == StatusMatched {
		httpx.WriteOK(w, map[string]any{
			"matchId":    e.MatchID,
			"status":     StatusMatched,
			"roomId":     e.RoomID,
			"serverId":   e.ServerID(),
			"wsUrl":      e.WsURL,
			"enterToken": e.EnterToken,
			"expireAt":   e.ExpireAt,
		})
		return
	}
	httpx.WriteOK(w, map[string]any{
		"matchId":              e.MatchID,
		"status":               e.Status,
		"stage":                "SEARCHING_ROOM",
		"waitSeconds":          e.WaitSeconds(),
		"estimatedWaitSeconds": h.cfg.MaxWaitSeconds,
	})
}

func (h *Handlers) config(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "classic"
	}
	httpx.WriteOK(w, map[string]any{
		"mode":                 mode,
		"maxPlayers":           h.cfg.MaxPlayers,
		"minPlayers":           h.cfg.MinStartPlayers,
		"estimatedWaitSeconds": h.cfg.MaxWaitSeconds,
		"maxWaitSeconds":       h.cfg.MaxWaitSeconds,
	})
}
