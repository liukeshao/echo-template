package middleware

import (
	"log/slog"
	"strings"

	"github.com/labstack/echo/v4"

	appContext "github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/services"
)

// extractTokenFromHeader 从Authorization header提取token
func extractTokenFromHeader(c echo.Context) (string, error) {
	// 从Authorization header获取token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.ErrUnauthorized.Errorf("缺少Authorization头")
	}

	// 检查Bearer格式
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", errors.ErrUnauthorized.Errorf("无效的Authorization格式")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == "" {
		return "", errors.ErrUnauthorized.Errorf("令牌不能为空")
	}

	return tokenString, nil
}

// RequireAuth 要求用户认证的中间件
func RequireAuth(authService *services.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			// 从header提取token
			tokenString, err := extractTokenFromHeader(c)
			if err != nil {
				slog.WarnContext(ctx, "认证失败：提取token失败", "error", err)
				return err
			}

			// 进行认证验证
			user, token, err := authService.AuthenticateUser(ctx, tokenString)
			if err != nil {
				return err
			}

			// 更新token使用时间
			authService.UpdateTokenUsage(token)

			// 将用户信息存储到context中
			ctx = appContext.WithUser(ctx, user)

			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
