// Package httpkit 提供基于标准库 net/http 的零依赖入站服务端原语套件。
package httpkit

import (
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/liukeshao/go-rest-template/pkg/httperr"
)

// Router 封装 *http.ServeMux、全局中间件链及 404/405 RFC 9457 兜底。
type Router struct {
	mux          *http.ServeMux
	errorEncoder httperr.Encoder
	middlewares  []Middleware

	once    sync.Once
	wrapped http.Handler
}

func NewRouter() *Router {
	return &Router{
		mux:          http.NewServeMux(),
		errorEncoder: httperr.DefaultEncoder,
	}
}

func (r *Router) SetErrorEncoder(enc httperr.Encoder) {
	if enc != nil {
		r.errorEncoder = enc
	}
}

func (r *Router) Use(mws ...Middleware) {
	for _, mw := range mws {
		if mw != nil {
			r.middlewares = append(r.middlewares, mw)
		}
	}
}

func (r *Router) Mount(handlers ...Handler) {
	for _, h := range handlers {
		if h != nil {
			h.Mount(r.mux)
		}
	}
}

func (r *Router) Handle(pattern string, h http.Handler) {
	r.mux.Handle(pattern, h)
}

func (r *Router) HandleFunc(pattern string, fn http.HandlerFunc) {
	r.mux.HandleFunc(pattern, fn)
}

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
				r.errorEncoder(w, req, httperr.Problem{
					Status: http.StatusMethodNotAllowed,
					Metadata: httperr.MethodInfo{
						Method:  req.Method,
						Path:    req.URL.Path,
						Allowed: allowed,
					},
				})
				return
			default:
				r.errorEncoder(w, req, httperr.Problem{
					Status: http.StatusNotFound,
					Metadata: httperr.ResourceInfo{
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
