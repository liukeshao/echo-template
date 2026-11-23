package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/httpruntime"
	"github.com/liukeshao/echo-template/pkg/services"
)

// extractTokenFromHeader 从 Authorization header 提取 Bearer token。
// header 缺失或格式非法时返回 AUTH_TOKEN_INVALID oops 错误（→ HTTP 401）。
func extractTokenFromHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", oops.
			Code(codes.AuthTokenInvalid.String()).
			Public("missing or invalid Authorization header").
			Errorf("missing or invalid Authorization header")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return "", oops.
			Code(codes.AuthTokenInvalid.String()).
			Public("bearer token is empty").
			Errorf("empty bearer token")
	}

	return token, nil
}

// RequireAuth 返回标准 net/http 鉴权中间件：验证 Bearer token，将已认证 *ent.User 写入 context 后放行。
// token 不写入 context，防止被日志框架意外序列化后泄露。
func RequireAuth(authService *services.AuthService, rt *httpruntime.Runtime) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			tokenString, err := extractTokenFromHeader(r)
			if err != nil {
				slog.WarnContext(ctx, "auth failed: extract token", "error", err)
				rt.WriteProblem(w, r, err)
				return
			}

			user, err := authService.AuthenticateUser(ctx, tokenString)
			if err != nil {
				rt.WriteProblem(w, r, err)
				return
			}

			next.ServeHTTP(w, r.WithContext(appctx.WithUser(ctx, user)))
		})
	}
}
