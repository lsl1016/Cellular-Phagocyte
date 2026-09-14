// Package httpx 提供所有 API 处理器使用的标准 HTTP JSON 响应封装：
// {code, message, data}。
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const DefaultMaxJSONBodyBytes int64 = 1 << 20 // 1 MiB，远高于当前 API 正常请求体大小。

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

// DecodeJSON 将请求体严格解码到 v：限制最大体积、拒绝未知字段，并拒绝尾随第二个 JSON 值。
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return DecodeJSONLimit(w, r, v, DefaultMaxJSONBodyBytes)
}

// DecodeJSONLimit 与 DecodeJSON 相同，但允许调用方覆盖请求体字节上限。
func DecodeJSONLimit(w http.ResponseWriter, r *http.Request, v any, maxBytes int64) bool {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxJSONBodyBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			WriteErr(w, http.StatusRequestEntityTooLarge, 50001, "request body too large")
			return false
		}
		WriteErr(w, http.StatusBadRequest, 50000, "invalid request body")
		return false
	}

	// 只允许一个 JSON 文档；空白之后必须直接 EOF。
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				WriteErr(w, http.StatusRequestEntityTooLarge, 50001, "request body too large")
				return false
			}
		}
		WriteErr(w, http.StatusBadRequest, 50000, "invalid request body")
		return false
	}
	return true
}
