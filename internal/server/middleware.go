package server

import (
	"net/http"
	"uuid"

	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/pkg/logging"
	"github.com/liukeshao/echo-template/pkg/rest"
)

// HeaderRequestID 是请求追踪 ID 使用的标准 HTTP 头。
const HeaderRequestID = "X-Request-ID"

// RequestID 读取或生成 UUID v7 格式的 X-Request-ID，写入响应头与 context（供 slog 和 Problem 复用）。
func RequestID() rest.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(HeaderRequestID)
			if rid == "" {
				rid = uuid.NewV7().String()
			}
			w.Header().Set(HeaderRequestID, rid)
			next.ServeHTTP(w, r.WithContext(logging.WithRequestID(r.Context(), rid)))
		})
	}
}

// Locale 协商请求语言（?lang= > Accept-Language > Default）并写入 context。
func Locale(tr i18n.Translator) rest.Middleware {
	if tr == nil {
		tr = noopTranslator{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tag := tr.Match(
				r.Header.Get("Accept-Language"),
				r.URL.Query().Get("lang"),
			)
			next.ServeHTTP(w, r.WithContext(i18n.WithLocale(r.Context(), tag)))
		})
	}
}
