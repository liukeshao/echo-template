package httpkit

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/liukeshao/echo-template/pkg/httperr"
)

// Authenticator 实现单个命名认证方案（与 TypeSpec @useAuth 名称对应，如 "BearerAuth"、"ApiKeyAuth"）。
// 认证成功时返回附加了身份信息的 context；失败时返回 error，由 httperr.Encoder 统一渲染。
type Authenticator func(ctx context.Context, r *http.Request) (context.Context, error)

// ExtractBearerToken 从 Authorization 请求头提取 Bearer token（scheme 大小写不敏感，遵循 RFC 9110）。
// header 缺失、scheme 非法或 token 为空时返回携带 PreconditionInfo 的 401 httperr.Problem。
func ExtractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if strings.TrimSpace(authHeader) == "" {
		return "", bearerHeaderProblem(httperr.PreconditionMissingHeader)
	}

	scheme, rawToken, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", bearerHeaderProblem(httperr.PreconditionMalformedHeader)
	}

	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", bearerHeaderProblem(httperr.PreconditionEmptyToken)
	}

	return token, nil
}

func bearerHeaderProblem(reason string) error {
	return httperr.Problem{
		Status: http.StatusUnauthorized,
		Metadata: httperr.PreconditionInfo{
			Type:    reason,
			Subject: "authorization",
			Detail:  "Bearer",
		},
	}
}

// BearerAuth 将纯 token 验证函数包装为 Authenticator：先按 RFC 9110/6750 提取 Bearer token，再交由 verify 校验并丰富 context。
func BearerAuth(verify func(ctx context.Context, token string) (context.Context, error)) Authenticator {
	return func(ctx context.Context, r *http.Request) (context.Context, error) {
		token, err := ExtractBearerToken(r)
		if err != nil {
			return ctx, err
		}
		return verify(ctx, token)
	}
}

// AuthMiddleware 返回基于命名方案映射的 RouteMiddleware，原生支持 TypeSpec @useAuth 的
// 单方案、AND 组合、OR 备选以及 NoAuth 可选认证。
// 若路由声明了未在 schemes 中注册的方案名，会在挂载阶段直接 panic（fail-fast）。
func AuthMiddleware(enc httperr.Encoder, schemes map[string]Authenticator) RouteMiddleware {
	if enc == nil {
		enc = httperr.DefaultEncoder
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
					panic(fmt.Sprintf("httpkit: operation %q requires unregistered auth scheme %q", op.ID, name))
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
