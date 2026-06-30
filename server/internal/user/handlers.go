package user

import (
	"net/http"
	"strings"

	"cellular-phagocyte/server/internal/httpx"
)

// Handlers 暴露用户/资产 HTTP API。
type Handlers struct {
	svc *Service
}

// NewHandlers 将 HTTP 处理器与用户服务关联。
func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// Register 在给定 mux 上挂载路由。
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/guest-login", h.guestLogin)
	mux.HandleFunc("GET /api/users/me", h.me)
	mux.HandleFunc("GET /api/assets/me", h.assetsMe)
}

type guestLoginReq struct {
	DeviceID      string `json:"deviceId"`
	ClientVersion string `json:"clientVersion"`
}

func (h *Handlers) guestLogin(w http.ResponseWriter, r *http.Request) {
	var req guestLoginReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	u, a, token := h.svc.GuestLogin(req.DeviceID)
	httpx.WriteOK(w, map[string]any{
		"accessToken": token,
		"user": map[string]any{
			"userId":   u.UserID,
			"nickname": u.Nickname,
			"avatar":   u.Avatar,
			"userType": u.UserType,
			"level":    a.Level,
			"coin":     a.Coin,
			"exp":      a.Exp,
		},
	})
}

// AuthUser 提取 Bearer 令牌并解析用户，失败时写入 401。
func (h *Handlers) AuthUser(w http.ResponseWriter, r *http.Request) (*User, bool) {
	token := bearerToken(r)
	if token == "" {
		httpx.WriteErr(w, http.StatusUnauthorized, 50001, "未登录")
		return nil, false
	}
	u, err := h.svc.UserByToken(token)
	if err != nil {
		httpx.WriteErr(w, http.StatusUnauthorized, 50002, "Token无效")
		return nil, false
	}
	return u, true
}

// TryAuthUser 尝试解析当前用户，不写入错误（用于可选登录的接口）。
func (h *Handlers) TryAuthUser(r *http.Request) (*User, bool) {
	token := bearerToken(r)
	if token == "" {
		return nil, false
	}
	u, err := h.svc.UserByToken(token)
	if err != nil {
		return nil, false
	}
	return u, true
}

func (h *Handlers) me(w http.ResponseWriter, r *http.Request) {
	u, ok := h.AuthUser(w, r)
	if !ok {
		return
	}
	a, _ := h.svc.GetAsset(u.UserID)
	httpx.WriteOK(w, map[string]any{
		"userId":       u.UserID,
		"nickname":     u.Nickname,
		"avatar":       u.Avatar,
		"userType":     u.UserType,
		"status":       u.Status,
		"level":        a.Level,
		"exp":          a.Exp,
		"nextLevelExp": NextLevelExp(a.Exp),
		"coin":         a.Coin,
	})
}

func (h *Handlers) assetsMe(w http.ResponseWriter, r *http.Request) {
	u, ok := h.AuthUser(w, r)
	if !ok {
		return
	}
	a, _ := h.svc.GetAsset(u.UserID)
	httpx.WriteOK(w, map[string]any{
		"userId":       a.UserID,
		"coin":         a.Coin,
		"exp":          a.Exp,
		"level":        a.Level,
		"nextLevelExp": NextLevelExp(a.Exp),
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const p = "Bearer "
	if strings.HasPrefix(h, p) {
		return strings.TrimPrefix(h, p)
	}
	return h
}
