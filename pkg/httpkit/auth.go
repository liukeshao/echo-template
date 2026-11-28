package httpkit

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/liukeshao/go-rest-template/pkg/httperr"
)

const (
	AuthMissingHeader   = "missing_header"
	AuthMalformedHeader = "malformed_header"
	AuthEmptyToken      = "empty_token"
)

// Authenticator 实现单个命名认证方案，认证成功时返回附加身份信息的 context。
type Authenticator func(ctx context.Context, r *http.Request) (context.Context, error)

// ExtractBearerToken 从 Authorization 请求头提取 Bearer token（scheme 大小写不敏感，RFC 9110）。
func ExtractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if strings.TrimSpace(authHeader) == "" {
		return "", bearerHeaderProblem(AuthMissingHeader)
	}

	scheme, rawToken, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", bearerHeaderProblem(AuthMalformedHeader)
	}

	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", bearerHeaderProblem(AuthEmptyToken)
	}

	return token, nil
}

func bearerHeaderProblem(reason string) error {
	return httperr.Problem{
		Status: http.StatusUnauthorized,
		Metadata: httperr.PreconditionInfo{
			Type:      reason,
			Subject:   "authorization",
			Challenge: "Bearer",
		},
	}
}

// BearerAuth 将纯 token 校验函数包装为 Authenticator。
func BearerAuth(verify func(ctx context.Context, token string) (context.Context, error)) Authenticator {
	return func(ctx context.Context, r *http.Request) (context.Context, error) {
		token, err := ExtractBearerToken(r)
		if err != nil {
			return ctx, err
		}
		return verify(ctx, token)
	}
}

// AuthMiddleware 按 TypeSpec @useAuth 声明编排命名认证方案（支持 AND/OR/NoAuth，未注册方案挂载时 panic）。
func AuthMiddleware(enc httperr.Encoder, schemes map[string]Authenticator) RouteMiddleware {
	if enc == nil {
		enc = httperr.DefaultEncoder
	}
	cloned := maps.Clone(schemes)

	return func(op Operation) Middleware {
		if !op.HasAuth() {
			return nil
		}

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
						// 空选项对应 TypeSpec NoAuth，允许匿名放行。
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
