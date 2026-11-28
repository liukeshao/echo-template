package httpkit

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

var (
	defaultCORSMethods = []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	}
	defaultCORSHeaders = []string{
		"Authorization",
		"Content-Type",
		"Accept-Language",
		HeaderRequestID,
	}
)

// CORSConfig 配置轻量零依赖 CORS 中间件；AllowedOrigins 为空时默认禁用（透传）。
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// CORS 返回符合浏览器规范的跨域中间件：支持精确源白名单或 "*" 通配符（与 AllowCredentials 互斥）、
// 自动维护 Vary 头并短路处理 OPTIONS 预检请求。
func CORS(cfg CORSConfig) Middleware {
	if len(cfg.AllowedOrigins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	allowAll := slices.Contains(cfg.AllowedOrigins, "*")
	allowedSet := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			allowedSet[trimmed] = struct{}{}
		}
	}

	methods := cfg.AllowedMethods
	if len(methods) == 0 {
		methods = defaultCORSMethods
	}
	allowMethodsHeader := strings.Join(methods, ", ")

	headers := cfg.AllowedHeaders
	if len(headers) == 0 {
		headers = defaultCORSHeaders
	}
	allowHeadersHeader := strings.Join(headers, ", ")
	exposeHeadersHeader := strings.Join(cfg.ExposedHeaders, ", ")

	var maxAgeHeader string
	if cfg.MaxAge > 0 {
		maxAgeHeader = strconv.FormatInt(int64(cfg.MaxAge/time.Second), 10)
	}

	// 按 Fetch 规范，Access-Control-Allow-Origin: * 不得与 Access-Control-Allow-Credentials: true 组合。
	allowCreds := cfg.AllowCredentials && !allowAll

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Add("Vary", "Origin")

			var allowOrigin string
			switch {
			case allowAll:
				allowOrigin = "*"
			case hasOrigin(allowedSet, origin):
				allowOrigin = origin
			default:
				if isCORSPreflight(r) {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			if allowCreds {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if isCORSPreflight(r) {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
				w.Header().Set("Access-Control-Allow-Methods", allowMethodsHeader)
				w.Header().Set("Access-Control-Allow-Headers", allowHeadersHeader)
				if maxAgeHeader != "" {
					w.Header().Set("Access-Control-Max-Age", maxAgeHeader)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if exposeHeadersHeader != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposeHeadersHeader)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasOrigin(set map[string]struct{}, origin string) bool {
	_, ok := set[origin]
	return ok
}

func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}
