package httpx

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// OriginPolicy 为 HTTP CORS 与 WebSocket Upgrade 提供同一套来源校验规则。
// 空 Origin 用于非浏览器客户端，允许通过；浏览器请求必须满足：
//   - 与请求 Host 同源；或
//   - 命中显式 AllowedOrigins；或
//   - 开启 AllowLoopback 且 Origin 来自本机回环地址。
type OriginPolicy struct {
	AllowedOrigins []string
	AllowLoopback  bool
}

// AllowsRequest 判断请求携带的 Origin 是否允许。
func (p OriginPolicy) AllowsRequest(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	// 浏览器同源判断：scheme 的差异由浏览器 Origin 语义负责，这里比较 Host:Port。
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}

	canonical := strings.ToLower(u.Scheme + "://" + u.Host)
	for _, allowed := range p.AllowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" {
			return true
		}
		if strings.ToLower(strings.TrimRight(allowed, "/")) == canonical {
			return true
		}
	}

	return p.AllowLoopback && isLoopbackHostname(u.Hostname())
}

func isLoopbackHostname(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
