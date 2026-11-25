package rest

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// BodyLimit 使用标准库 http.MaxBytesReader 限制单次请求体最大字节数。
func BodyLimit(maxBytes int64) Middleware {
	if maxBytes <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// Recover 捕获下游 panic，记录堆栈日志并通过 enc 返回标准 500 错误响应。
func Recover(enc ErrorEncoder) Middleware {
	if enc == nil {
		enc = DefaultErrorEncoder
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.ErrorContext(r.Context(), "panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					enc(w, r, fmt.Errorf("panic: %v", rec))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// AccessLog 使用 slog 记录每个 HTTP 请求的方法、路径、状态码与耗时。
func AccessLog() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusCaptureWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			slog.InfoContext(r.Context(), "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"latency", time.Since(start),
			)
		})
	}
}

type statusCaptureWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusCaptureWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusCaptureWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap 暴露底层 http.ResponseWriter，兼容 Go 1.20+ http.ResponseController（Flusher / Hijacker 等）。
func (w *statusCaptureWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
