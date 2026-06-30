// Package httpx 提供所有 API 处理器使用的标准 HTTP JSON 响应封装：
// {code, message, data}。
package httpx

import (
	"encoding/json"
	"net/http"
)

// Response 是标准 API 封装。
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// WriteOK 写入成功封装（code 0）及给定数据。
func WriteOK(w http.ResponseWriter, data any) {
	write(w, http.StatusOK, Response{Code: 0, Message: "success", Data: data})
}

// WriteErr 写入带业务码和消息的错误封装。
func WriteErr(w http.ResponseWriter, httpStatus, code int, message string) {
	write(w, httpStatus, Response{Code: code, Message: message})
}

func write(w http.ResponseWriter, status int, r Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(r)
}

// DecodeJSON 将请求体解码到 v，失败时返回 false 并写入 400 错误。
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		WriteErr(w, http.StatusBadRequest, 50000, "invalid request body")
		return false
	}
	return true
}
