package rest

import (
	"context"
	"fmt"
	"maps"
	"net/http"
)

// Authenticator 实现单个命名认证方案（与 TypeSpec @useAuth 名称对应，如 "BearerAuth"、"ApiKeyAuth"）。
// 认证成功时返回附加了身份信息的 context；失败时返回 error，由 ErrorEncoder 统一渲染。
type Authenticator func(ctx context.Context, r *http.Request) (context.Context, error)

// AuthMiddleware 返回基于命名方案映射的 RouteMiddleware，原生支持 TypeSpec @useAuth 的
// 单方案、AND 组合、OR 备选以及 NoAuth 可选认证。
// 若路由声明了未在 schemes 中注册的方案名，会在挂载阶段直接 panic（fail-fast）。
func AuthMiddleware(enc ErrorEncoder, schemes map[string]Authenticator) RouteMiddleware {
	if enc == nil {
		enc = DefaultErrorEncoder
	}
	cloned := maps.Clone(schemes)

	return func(op Operation) Middleware {
		if !op.HasAuth() {
			return nil
		}

		// 挂载阶段校验所有声明的 scheme 均已注册，避免配置拼写错误或漏注册导致运行期静默放行。
		for _, alt := range op.Auth {
			for _, name := range alt {
				if cloned[name] == nil {
					panic(fmt.Sprintf("rest: operation %q requires unregistered auth scheme %q", op.ID, name))
				}
			}
		}

		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var lastErr error
				for _, alt := range op.Auth {
					if len(alt) == 0 {
						// 空选项对应 TypeSpec NoAuth（如 @useAuth(BearerAuth | NoAuth)），允许匿名继续。
						next.ServeHTTP(w, r)
						return
					}

					ctx := r.Context()
					var altErr error
					for _, name := range alt {
						var err error
						ctx, err = cloned[name](ctx, r)
						if err != nil {
							altErr = err
							break
						}
					}
					if altErr == nil {
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					lastErr = altErr
				}
				enc(w, r, lastErr)
			})
		}
	}
}
