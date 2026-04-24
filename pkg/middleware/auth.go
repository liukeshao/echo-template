package middleware

import (
	"log/slog"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/services"
)

// AuthMiddleware 是注册到 DI 容器的 echo.MiddlewareFunc 命名类型。
// 使用独立类型而非 echo.MiddlewareFunc 别名，避免与其他 MiddlewareFunc 冲突。
type AuthMiddleware echo.MiddlewareFunc

// NewAuthMiddleware 从 DI 容器构造 AuthMiddleware，供 BookHandler / MeHandler 注入。
func NewAuthMiddleware(i do.Injector) (AuthMiddleware, error) {
	authService := do.MustInvoke[*services.AuthService](i)
	return AuthMiddleware(RequireAuth(authService)), nil
}

// extractTokenFromHeader 从 Authorization header 提取 Bearer token。
// header 缺失或格式非法时返回 AUTH_TOKEN_INVALID oops 错误，
// 由 error_handler 统一映射为 401 Unauthenticated。
func extractTokenFromHeader(c *echo.Context) (string, error) {
	authHeader := c.Request().Header.Get("Authorization")
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

// RequireAuth 验证 Bearer token，将已认证用户写入 context 后放行。
// token 不写入 context，防止被日志框架意外序列化后泄露。
// token 缺失/无效直接返回 AUTH_TOKEN_INVALID（→ 401）。
func RequireAuth(authService *services.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()

			tokenString, err := extractTokenFromHeader(c)
			if err != nil {
				slog.WarnContext(ctx, "auth failed: extract token", "error", err)
				return err
			}

			user, err := authService.AuthenticateUser(ctx, tokenString)
			if err != nil {
				return err
			}

			c.SetRequest(c.Request().WithContext(appctx.WithUser(ctx, user)))
			return next(c)
		}
	}
}
