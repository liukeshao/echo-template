package rest

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
	"uuid"

	"github.com/liukeshao/echo-template/pkg/appctx"
)

// HeaderRequestID 是请求追踪 ID 使用的标准 HTTP 头。
const HeaderRequestID = "X-Request-ID"

// Router 封装标准库 *http.ServeMux、Runtime、全局中间件链以及 404/405 RFC 9457 兜底。
// Router 自身实现 http.Handler，可直接传入 rest.NewServer 或用于测试。
type Router struct {
	mux          *http.ServeMux
	rt           *Runtime
	maxBodyBytes int64

	once    sync.Once
	wrapped http.Handler
}

// NewRouter 创建 Router 实例。
func NewRouter(rt *Runtime, opts ...RouterOption) *Router {
	if rt == nil {
		rt = DefaultRuntime()
	}
	r := &Router{
		mux: http.NewServeMux(),
		rt:  rt,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// Mount 将一个或多个 rest.Handler（如 api.NewBooksHandler(...)）挂载到 Router。
func (r *Router) Mount(handlers ...Handler) {
	for _, h := range handlers {
		if h != nil {
			h.Mount(r.mux)
		}
	}
}

// Handle 直接向底层 *http.ServeMux 注册原生 http.Handler。
func (r *Router) Handle(pattern string, h http.Handler) {
	r.mux.Handle(pattern, h)
}

// HandleFunc 直接向底层 *http.ServeMux 注册原生 http.HandlerFunc。
func (r *Router) HandleFunc(pattern string, fn http.HandlerFunc) {
	r.mux.HandleFunc(pattern, fn)
}

// ServeHTTP 按全局中间件链执行请求分发：
// RequestID → Locale → AccessLog → Recover → BodyLimit → 404/405 RFC 9457 拦截 → ServeMux。
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.once.Do(func() {
		r.wrapped = r.buildHandler()
	})
	r.wrapped.ServeHTTP(w, req)
}

func (r *Router) buildHandler() http.Handler {
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h, pattern := r.mux.Handler(req)
		if pattern == "" {
			probe := &statusProbeWriter{header: make(http.Header)}
			h.ServeHTTP(probe, req)
			if probe.status == http.StatusMethodNotAllowed {
				if allow := probe.header.Get("Allow"); allow != "" {
					w.Header().Set("Allow", allow)
				}
				r.rt.WriteProblem(w, req, Problem{
					Status: http.StatusMethodNotAllowed,
					Code:   "METHOD_NOT_ALLOWED",
				})
				return
			}
			r.rt.WriteProblem(w, req, Problem{
				Status: http.StatusNotFound,
				Code:   "NOT_FOUND",
			})
			return
		}
		// 必须调用 mux.ServeHTTP 才会填充 Go 1.22+ 的 req.PathValue(...)。
		r.mux.ServeHTTP(w, req)
	})

	if r.maxBodyBytes > 0 {
		handler = BodyLimit(r.maxBodyBytes)(handler)
	}
	handler = r.rt.Recover()(handler)
	handler = AccessLog()(handler)
	handler = r.rt.Locale()(handler)
	handler = RequestID()(handler)
	return handler
}

// BodyLimit 使用标准库 http.MaxBytesReader 限制单次请求体最大字节数。
func BodyLimit(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID 读取或生成 UUID v7 格式的 X-Request-ID，写入响应头与 context（供 slog 和 Problem 复用）。
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(HeaderRequestID)
			if rid == "" {
				rid = uuid.NewV7().String()
			}
			w.Header().Set(HeaderRequestID, rid)
			next.ServeHTTP(w, r.WithContext(appctx.WithRequestID(r.Context(), rid)))
		})
	}
}

// Locale 协商请求语言（?lang= > Accept-Language > Default）并写入 context。
func (rt *Runtime) Locale() Middleware {
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
func (rt *Runtime) Recover() Middleware {
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
