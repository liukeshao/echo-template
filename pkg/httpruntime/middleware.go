package httpruntime

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// HeaderRequestID 是请求追踪 ID 使用的标准 HTTP 头。
const HeaderRequestID = "X-Request-ID"

// WrapMux 为标准 *http.ServeMux 挂载完整全局中间件链：
// Recover → RequestID → Locale → AccessLog → BodyLimit → 404/405 RFC 9457 拦截。
func (rt *Runtime) WrapMux(mux *http.ServeMux, maxBodyBytes int64) http.Handler {
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern == "" {
			probe := &statusProbeWriter{header: make(http.Header)}
			h.ServeHTTP(probe, r)
			if probe.status == http.StatusMethodNotAllowed {
				if allow := probe.header.Get("Allow"); allow != "" {
					w.Header().Set("Allow", allow)
				}
				rt.WriteProblem(w, r, Problem{
					Status: http.StatusMethodNotAllowed,
					Code:   "METHOD_NOT_ALLOWED",
					Detail: http.StatusText(http.StatusMethodNotAllowed),
				})
				return
			}
			rt.WriteProblem(w, r, Problem{
				Status: http.StatusNotFound,
				Code:   "NOT_FOUND",
				Detail: http.StatusText(http.StatusNotFound),
			})
			return
		}
		// 必须调用 mux.ServeHTTP 才会填充 Go 1.22+ 的 r.PathValue(...)。
		mux.ServeHTTP(w, r)
	})

	if maxBodyBytes > 0 {
		handler = BodyLimit(maxBodyBytes)(handler)
	}
	handler = AccessLog()(handler)
	handler = rt.Locale()(handler)
	handler = RequestID()(handler)
	handler = rt.Recover()(handler)
	return handler
}

// BodyLimit 使用标准库 http.MaxBytesReader 限制单次请求体最大字节数。
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID 读取或生成 ULID 格式的 X-Request-ID，写入响应头与 context（供 slog 和 Problem 复用）。
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(HeaderRequestID)
			if rid == "" {
				rid = utils.GenerateULID()
			}
			w.Header().Set(HeaderRequestID, rid)
			next.ServeHTTP(w, r.WithContext(appctx.WithRequestID(r.Context(), rid)))
		})
	}
}

// Locale 协商请求语言（?lang= > Accept-Language > Default）并写入 context。
func (rt *Runtime) Locale() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tag := rt.translator.Match(
				r.Header.Get("Accept-Language"),
				r.URL.Query().Get("lang"),
			)
			next.ServeHTTP(w, r.WithContext(appctx.WithLocale(r.Context(), tag)))
		})
	}
}

// Recover 捕获下游 panic，记录堆栈日志并返回标准 500 RFC 9457 Problem。
func (rt *Runtime) Recover() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.ErrorContext(r.Context(), "panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					rt.WriteProblem(w, r, fmt.Errorf("panic: %v", rec))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// AccessLog 使用 slog 记录每个 HTTP 请求的方法、路径、状态码与耗时。
func AccessLog() func(http.Handler) http.Handler {
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

func (w *statusCaptureWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

type statusProbeWriter struct {
	header http.Header
	status int
}

func (p *statusProbeWriter) Header() http.Header         { return p.header }
func (p *statusProbeWriter) WriteHeader(code int)        { p.status = code }
func (p *statusProbeWriter) Write(b []byte) (int, error) { return len(b), nil }
