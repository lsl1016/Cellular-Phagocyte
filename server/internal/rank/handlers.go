package rank

import (
	"net/http"
	"strconv"

	"cellular-phagocyte/server/internal/httpx"
	"cellular-phagocyte/server/internal/user"
)

const (
	defaultPageSize = 50
	pageSizeLimit   = 100
)

// Handlers 暴露排行榜查询接口。
type Handlers struct {
	svc  *Service
	auth *user.Handlers
}

// NewHandlers 关联排行榜 HTTP 处理器。
func NewHandlers(svc *Service, auth *user.Handlers) *Handlers {
	return &Handlers{svc: svc, auth: auth}
}

// Register 挂载排行榜路由。
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/ranks", h.top)
	mux.HandleFunc("GET /api/ranks/me", h.me)
	mux.HandleFunc("GET /api/ranks/config", h.config)
}

func (h *Handlers) top(w http.ResponseWriter, r *http.Request) {
	rankType := r.URL.Query().Get("rankType")
	if rankType == "" {
		rankType = TypeDaily
	}
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	pageSize := atoiDefault(r.URL.Query().Get("pageSize"), defaultPageSize)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > pageSizeLimit {
		pageSize = defaultPageSize
	}

	// 可选登录：用于标记自身名次
	selfID := ""
	if u, ok := h.auth.TryAuthUser(r); ok {
		selfID = u.UserID
	}

	res, ok := h.svc.Top(rankType, selfID, page, pageSize)
	if !ok {
		httpx.WriteErr(w, http.StatusBadRequest, 48001, "榜单类型不存在")
		return
	}
	httpx.WriteOK(w, res)
}

func (h *Handlers) me(w http.ResponseWriter, r *http.Request) {
	u, ok := h.auth.AuthUser(w, r)
	if !ok {
		return
	}
	rankType := r.URL.Query().Get("rankType")
	if rankType == "" {
		rankType = TypeDaily
	}
	self, ok := h.svc.Me(rankType, u.UserID)
	if !ok {
		httpx.WriteErr(w, http.StatusBadRequest, 48001, "榜单类型不存在")
		return
	}
	httpx.WriteOK(w, map[string]any{
		"rankType": rankType, "rank": self.Rank, "score": self.Score, "onRank": self.OnRank,
	})
}

func (h *Handlers) config(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteOK(w, map[string]any{
		"rankTypes": []map[string]any{
			{"rankType": TypeDaily, "name": "日榜", "enabled": true},
			{"rankType": TypeWeekly, "name": "周榜", "enabled": true},
			{"rankType": TypeBestScore, "name": "最高分榜", "enabled": true},
		},
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
