package middleware

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// DocsConfig 文档中间件配置
type DocsConfig struct {
	AppConfig *config.Config
}

// DocsMiddleware 文档访问中间件
type DocsMiddleware struct {
	config DocsConfig
}

// NewDocsMiddleware 创建文档中间件
func NewDocsMiddleware(cfg DocsConfig) *DocsMiddleware {
	return &DocsMiddleware{config: cfg}
}

// CheckAccess 检查文档访问权限
func (m *DocsMiddleware) CheckAccess(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		// 检查文档是否启用
		if !m.config.AppConfig.App.Docs.Enabled {
			slog.WarnContext(ctx, "尝试访问已禁用的API文档",
				"ip", c.RealIP(),
				"user_agent", c.Request().UserAgent(),
				"path", c.Request().URL.Path,
			)

			return errors.ErrNotFound("页面不存在")
		}

		// 生产环境额外检查
		if m.config.AppConfig.App.Environment == config.EnvProduction ||
			string(m.config.AppConfig.App.Environment) == "prod" {
			slog.ErrorContext(ctx, "生产环境下尝试访问API文档",
				"ip", c.RealIP(),
				"user_agent", c.Request().UserAgent(),
				"path", c.Request().URL.Path,
			)

			// 生产环境返回404，不暴露存在文档
			return echo.NewHTTPError(http.StatusNotFound, "页面不存在")
		}

		// 记录文档访问日志
		slog.InfoContext(ctx, "访问API文档",
			"environment", m.config.AppConfig.App.Environment,
			"ip", c.RealIP(),
			"path", c.Request().URL.Path,
		)

		return next(c)
	}
}

// SecurityHeaders 为文档页面添加安全头
func (m *DocsMiddleware) SecurityHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// 设置安全响应头
		c.Response().Header().Set("X-Frame-Options", "DENY")
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
		c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// 非生产环境允许开发者工具
		if m.config.AppConfig.App.Environment != config.EnvProduction {
			c.Response().Header().Set("Content-Security-Policy",
				"default-src 'self' 'unsafe-inline' 'unsafe-eval' unpkg.com; "+
					"script-src 'self' 'unsafe-inline' 'unsafe-eval' unpkg.com; "+
					"style-src 'self' 'unsafe-inline' unpkg.com; "+
					"connect-src 'self'; "+
					"img-src 'self' data:;")
		}

		return next(c)
	}
}
