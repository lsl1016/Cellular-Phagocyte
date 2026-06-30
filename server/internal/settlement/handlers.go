package settlement

import (
	"net/http"

	"cellular-phagocyte/server/internal/httpx"
	"cellular-phagocyte/server/internal/user"
)

// Handlers 暴露结算查询接口。
type Handlers struct {
	svc  *Service
	auth *user.Handlers
}

// NewHandlers 关联结算 HTTP 处理器。
func NewHandlers(svc *Service, auth *user.Handlers) *Handlers {
	return &Handlers{svc: svc, auth: auth}
}

// Register 挂载结算路由。
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settlements/latest", h.latest)
	mux.HandleFunc("GET /api/settlements/{roomId}/me", h.byRoom)
}

func (h *Handlers) byRoom(w http.ResponseWriter, r *http.Request) {
	u, ok := h.auth.AuthUser(w, r)
	if !ok {
		return
	}
	roomID := r.PathValue("roomId")
	res, found := h.svc.Result(roomID, u.UserID)
	if !found {
		httpx.WriteErr(w, http.StatusNotFound, 46002, "结算结果不存在")
		return
	}
	httpx.WriteOK(w, res)
}

func (h *Handlers) latest(w http.ResponseWriter, r *http.Request) {
	u, ok := h.auth.AuthUser(w, r)
	if !ok {
		return
	}
	res, found := h.svc.LatestResult(u.UserID)
	if !found {
		httpx.WriteErr(w, http.StatusNotFound, 46002, "结算结果不存在")
		return
	}
	httpx.WriteOK(w, res)
}
