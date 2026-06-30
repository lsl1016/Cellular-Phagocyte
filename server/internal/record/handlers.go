package record

import (
	"net/http"
	"strconv"

	"cellular-phagocyte/server/internal/httpx"
	"cellular-phagocyte/server/internal/user"
)

// Handlers 暴露战绩查询接口。
type Handlers struct {
	svc  *Service
	auth *user.Handlers
}

// NewHandlers 关联战绩 HTTP 处理器。
func NewHandlers(svc *Service, auth *user.Handlers) *Handlers {
	return &Handlers{svc: svc, auth: auth}
}

// Register 挂载战绩路由。注意：更具体的路径需先于 /{roomId} 注册。
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/records", h.list)
	mux.HandleFunc("GET /api/records/summary", h.summary)
	mux.HandleFunc("GET /api/records/latest-settlement", h.latest)
	mux.HandleFunc("GET /api/records/{roomId}", h.detail)
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	u, ok := h.auth.AuthUser(w, r)
	if !ok {
		return
	}
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	pageSize := atoiDefault(r.URL.Query().Get("pageSize"), 10)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	mode := r.URL.Query().Get("mode")

	total, list := h.svc.List(u.UserID, mode, page, pageSize)
	httpx.WriteOK(w, map[string]any{
		"page": page, "pageSize": pageSize, "total": total, "list": list,
	})
}

func (h *Handlers) detail(w http.ResponseWriter, r *http.Request) {
	u, ok := h.auth.AuthUser(w, r)
	if !ok {
		return
	}
	roomID := r.PathValue("roomId")
	e, found := h.svc.Get(roomID, u.UserID)
	if !found {
		httpx.WriteErr(w, http.StatusNotFound, 47001, "战绩不存在")
		return
	}
	httpx.WriteOK(w, e)
}

func (h *Handlers) summary(w http.ResponseWriter, r *http.Request) {
	u, ok := h.auth.AuthUser(w, r)
	if !ok {
		return
	}
	httpx.WriteOK(w, h.svc.Summary(u.UserID))
}

func (h *Handlers) latest(w http.ResponseWriter, r *http.Request) {
	u, ok := h.auth.AuthUser(w, r)
	if !ok {
		return
	}
	e, found := h.svc.Latest(u.UserID)
	if !found {
		httpx.WriteOK(w, map[string]any{"hasLatest": false})
		return
	}
	httpx.WriteOK(w, map[string]any{
		"hasLatest": true, "viewed": false, "roomId": e.RoomID, "rank": e.Rank,
		"totalPlayers": e.TotalPlayers, "finalScore": e.FinalScore,
		"coinReward": e.CoinReward, "expReward": e.ExpReward, "endTime": e.EndTime,
	})
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
