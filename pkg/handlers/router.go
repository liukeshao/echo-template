package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/services"
)

// BuildRouter builds the router.
func BuildRouter(c *services.Container) error {
	// 设置自定义错误处理器
	c.Web.HTTPErrorHandler = AppErrorHandler

	// Non-static file route group.
	g := c.Web.Group("")

	g.Use(
		echomw.RemoveTrailingSlashWithConfig(echomw.TrailingSlashConfig{
			RedirectCode: http.StatusMovedPermanently,
		}),
		echomw.Recover(),
		echomw.RequestIDWithConfig(echomw.RequestIDConfig{
			RequestIDHandler: func(c echo.Context, s string) {
				ctx := c.Request().Context()
				ctx = appctx.WithRequestID(ctx, s)
				c.SetRequest(c.Request().WithContext(ctx))
			},
		}),
		echomw.Gzip(),
		echomw.TimeoutWithConfig(echomw.TimeoutConfig{
			Timeout: c.Config.App.Timeout,
		}),
	)

	// Initialize and register all handlers.
	for _, h := range GetHandlers() {
		if err := h.Init(c); err != nil {
			return err
		}

		h.Routes(g)
	}

	return nil
}
