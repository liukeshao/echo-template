package httpkit

import (
	"net/http"
	"slices"
	"sync"

	"github.com/liukeshao/go-rest-template/pkg/httperr"
)

type Middleware = func(http.Handler) http.Handler

// Handler 同时实现标准 http.Handler 与可展平挂载到共享 *http.ServeMux 的 Mount 接口。
type Handler interface {
	http.Handler
	Mount(mux *http.ServeMux)
}

type mountableHandler struct {
	register func(mux *http.ServeMux)

	once sync.Once
	mux  *http.ServeMux
}

func NewHandler(register func(mux *http.ServeMux)) Handler {
	return &mountableHandler{register: register}
}

func (h *mountableHandler) Mount(mux *http.ServeMux) {
	h.register(mux)
}

func (h *mountableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.once.Do(func() {
		h.mux = http.NewServeMux()
		h.register(h.mux)
	})
	h.mux.ServeHTTP(w, r)
}

// Operation 描述单个 HTTP 路由操作元数据；Auth 按析取范式（DNF）表示 @useAuth 认证组合。
type Operation struct {
	ID      string
	Method  string
	Pattern string
	Auth    [][]string
}

func (o Operation) HasAuth() bool { return len(o.Auth) > 0 }

// RouteMiddleware 按操作元数据返回标准 Middleware；返回 nil 表示跳过该路由。
type RouteMiddleware func(op Operation) Middleware

type HandlerOption func(*HandlerConfig)

type HandlerConfig struct {
	ErrorEncoder httperr.Encoder
	Middlewares  []RouteMiddleware

	authSchemes  map[string]Authenticator
	externalAuth bool
}

func WithErrorEncoder(enc httperr.Encoder) HandlerOption {
	return func(c *HandlerConfig) {
		if enc != nil {
			c.ErrorEncoder = enc
		}
	}
}

func WithAuthScheme(name string, auth Authenticator) HandlerOption {
	return func(c *HandlerConfig) {
		if name == "" || auth == nil {
			return
		}
		if c.authSchemes == nil {
			c.authSchemes = make(map[string]Authenticator)
		}
		c.authSchemes[name] = auth
	}
}

// WithExternalAuth 显式声明 @useAuth 由外部 WithMiddleware 接管，跳过内置 AuthMiddleware 的 scheme 注册检查。
// 未显式调用此选项时保持 fail-closed：任何带有 @useAuth 的操作若未通过 WithAuthScheme 注册对应方案，挂载时即 panic。
func WithExternalAuth() HandlerOption {
	return func(c *HandlerConfig) {
		c.externalAuth = true
	}
}

func WithMiddleware(mws ...RouteMiddleware) HandlerOption {
	return func(c *HandlerConfig) {
		for _, mw := range mws {
			if mw != nil {
				c.Middlewares = append(c.Middlewares, mw)
			}
		}
	}
}

func ApplyHandlerOptions(opts ...HandlerOption) HandlerConfig {
	var cfg HandlerConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.ErrorEncoder == nil {
		cfg.ErrorEncoder = httperr.DefaultEncoder
	}
	if !cfg.externalAuth {
		// 命名鉴权始终置于用户 WithMiddleware 外层且默认 fail-closed，避免漏传 WithAuthScheme 时受保护路由静默放行。
		cfg.Middlewares = append([]RouteMiddleware{AuthMiddleware(cfg.ErrorEncoder, cfg.authSchemes)}, cfg.Middlewares...)
	}
	return cfg
}

func (c HandlerConfig) Wrap(op Operation, fn http.HandlerFunc) http.Handler {
	var h http.Handler = fn
	for _, v := range slices.Backward(c.Middlewares) {
		if mw := v(op); mw != nil {
			h = mw(h)
		}
	}
	return h
}

func (c HandlerConfig) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if c.ErrorEncoder != nil {
		c.ErrorEncoder(w, r, err)
		return
	}
	httperr.DefaultEncoder(w, r, err)
}
