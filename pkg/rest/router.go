package rest

import (
	"net/http"
	"slices"
	"strings"
	"sync"
)

// RouterOption 配置 rest.Router。
type RouterOption func(*Router)

// WithRouterErrorEncoder 设置 Router 用于 404/405 兜底响应的 ErrorEncoder。
func WithRouterErrorEncoder(enc ErrorEncoder) RouterOption {
	return func(r *Router) {
		if enc != nil {
			r.errorEncoder = enc
		}
	}
}

// Router 封装标准库 *http.ServeMux、可组合的全局中间件链以及 404/405 RFC 9457 兜底
// （同时保留 Go ServeMux 原生的 301/307/308 路径规范化重定向）。
type Router struct {
	mux          *http.ServeMux
	errorEncoder ErrorEncoder
	middlewares  []Middleware

	once    sync.Once
	wrapped http.Handler
}

// NewRouter 创建 Router 实例（零配置默认使用 DefaultErrorEncoder）。
func NewRouter(opts ...RouterOption) *Router {
	r := &Router{
		mux:          http.NewServeMux(),
		errorEncoder: DefaultErrorEncoder,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// Use 追加全局中间件（按注册顺序由外到内执行，覆盖所有已注册路由及 404/405 兜底）。
func (r *Router) Use(mws ...Middleware) {
	for _, mw := range mws {
		if mw != nil {
			r.middlewares = append(r.middlewares, mw)
		}
	}
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

// ServeHTTP 执行已注册的全局中间件链，并将请求分发至底层 *http.ServeMux。
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
			switch probe.status {
			case http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
				// 保留 Go ServeMux 对路径清理/尾部斜杠的 301/307/308 重定向（含 Location 头）。
				h.ServeHTTP(w, req)
				return
			case http.StatusMethodNotAllowed:
				allow := probe.header.Get("Allow")
				var allowed []string
				if allow != "" {
					w.Header().Set("Allow", allow)
					for item := range strings.SplitSeq(allow, ",") {
						if m := strings.TrimSpace(item); m != "" {
							allowed = append(allowed, m)
						}
					}
				}
				r.errorEncoder(w, req, Problem{
					Status: http.StatusMethodNotAllowed,
					Code:   CodeMethodNotAllowed,
					Metadata: MethodInfo{
						Method:  req.Method,
						Path:    req.URL.Path,
						Allowed: allowed,
					},
				})
				return
			default:
				r.errorEncoder(w, req, Problem{
					Status: http.StatusNotFound,
					Code:   CodeNotFound,
					Metadata: ResourceInfo{
						ResourceType: "route",
						ResourceName: req.URL.Path,
					},
				})
				return
			}
		}
		// 必须调用 mux.ServeHTTP 才会填充 Go 1.22+ 的 req.PathValue(...)。
		r.mux.ServeHTTP(w, req)
	})

	for _, v := range slices.Backward(r.middlewares) {
		handler = v(handler)
	}
	return handler
}

type statusProbeWriter struct {
	header http.Header
	status int
}

func (p *statusProbeWriter) Header() http.Header         { return p.header }
func (p *statusProbeWriter) WriteHeader(code int)        { p.status = code }
func (p *statusProbeWriter) Write(b []byte) (int, error) { return len(b), nil }
