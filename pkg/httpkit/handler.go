package httpkit

import (
	"net/http"
	"slices"
	"sync"

	"github.com/liukeshao/echo-template/pkg/httperr"
)

// Middleware 是标准 Go HTTP 中间件签名。
type Middleware = func(http.Handler) http.Handler

// Handler 表示一组由 TypeSpec 生成（或手写）的 HTTP 端点（Connect-Go 风格）：
//   - 本身直接实现标准 http.Handler，可独立运行于 httptest 或嵌入任意 HTTP 服务；
//   - 同时实现 Mount(*http.ServeMux)，可由 Router 展平挂载到共享 ServeMux，
//     完整保留 Go 1.22+ 精确路由匹配、冲突检测与 404/405 语义。
type Handler interface {
	http.Handler
	Mount(mux *http.ServeMux)
}

type mountableHandler struct {
	register func(mux *http.ServeMux)

	once sync.Once
	mux  *http.ServeMux
}

// NewHandler 将路由注册函数包装为同时支持独立 ServeHTTP 与 Mount 的 httpkit.Handler。
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

// Operation 描述由 TypeSpec 声明的单个 HTTP 操作元数据，
// 供路由级中间件（鉴权、限流、RBAC、审计、指标打点等）按需决策，无需改动框架或生成代码。
type Operation struct {
	ID      string
	Method  string
	Pattern string
	// Auth 按 OpenAPI / TypeSpec @useAuth 的析取范式（DNF）表示认证要求：
	//   - nil / 空切片                             ：公开路由（无需认证）
	//   - [][]string{{"BearerAuth"}}               ：需要 BearerAuth
	//   - [][]string{{"BearerAuth"}, {"ApiKeyAuth"}}：BearerAuth OR ApiKeyAuth
	//   - [][]string{{"BearerAuth", "ApiKeyAuth"}} ：BearerAuth AND ApiKeyAuth
	//   - [][]string{{"BearerAuth"}, {}}           ：可选认证（BearerAuth OR 匿名放行）
	Auth [][]string
}

// HasAuth 报告该操作是否声明了认证要求。
func (o Operation) HasAuth() bool { return len(o.Auth) > 0 }

// RouteMiddleware 根据操作元数据返回标准 Middleware；返回 nil 表示跳过该路由。
// 可直接用于接入任意第三方 net/http 中间件（如 Clerk / Auth0 / Casbin / 限流 / 指标）。
type RouteMiddleware func(op Operation) Middleware

// HandlerOption 配置由 TypeSpec 生成的 api.NewXxxHandler（类似 Connect-Go HandlerOption）。
type HandlerOption func(*HandlerConfig)

// HandlerConfig 保存生成 Handler 运行所需的错误编码器与路由级中间件链。
type HandlerConfig struct {
	ErrorEncoder httperr.Encoder
	Middlewares  []RouteMiddleware

	authSchemes map[string]Authenticator
}

// WithErrorEncoder 指定自定义错误编码器（例如接入业务错误与 i18n 的 RFC 9457 编码器）。
func WithErrorEncoder(enc httperr.Encoder) HandlerOption {
	return func(c *HandlerConfig) {
		if enc != nil {
			c.ErrorEncoder = enc
		}
	}
}

// WithAuthScheme 注册一个与 TypeSpec @useAuth 名称对应的认证方案（如 "BearerAuth"、"ApiKeyAuth"）。
// 可多次调用以同时支持多种认证方式（含 OR / AND 组合）；如要完全替换为第三方 net/http 鉴权中间件，
// 可不调用此项而改用 WithMiddleware。
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

// WithMiddleware 追加路由级中间件工厂（按注册顺序由外到内执行）。
// 既可用于接入任意第三方鉴权插件，也可用于按操作粒度配置限流、RBAC、审计等。
func WithMiddleware(mws ...RouteMiddleware) HandlerOption {
	return func(c *HandlerConfig) {
		for _, mw := range mws {
			if mw != nil {
				c.Middlewares = append(c.Middlewares, mw)
			}
		}
	}
}

// ApplyHandlerOptions 应用所有 HandlerOption 并补全默认 httperr.DefaultEncoder（零配置也能安全运行）。
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
	if len(cfg.authSchemes) > 0 {
		// 内置命名方案鉴权置于用户 WithMiddleware 外层，便于后续 RBAC/审计中间件读取已认证身份。
		// 若需自定义顺序（如限流在鉴权之前），可直接使用 WithMiddleware(..., AuthMiddleware(enc, schemes), ...)。
		cfg.Middlewares = append([]RouteMiddleware{AuthMiddleware(cfg.ErrorEncoder, cfg.authSchemes)}, cfg.Middlewares...)
	}
	return cfg
}

// Wrap 按注册顺序将匹配的路由级中间件包裹到 fn 外层（外层先执行）。
func (c HandlerConfig) Wrap(op Operation, fn http.HandlerFunc) http.Handler {
	var h http.Handler = fn
	for _, v := range slices.Backward(c.Middlewares) {
		if mw := v(op); mw != nil {
			h = mw(h)
		}
	}
	return h
}

// WriteError 使用已配置的 ErrorEncoder 写出错误响应（未配置时回落到 httperr.DefaultEncoder）。
func (c HandlerConfig) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if c.ErrorEncoder != nil {
		c.ErrorEncoder(w, r, err)
		return
	}
	httperr.DefaultEncoder(w, r, err)
}
