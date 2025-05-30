package handlers

import (
	"net/http"

	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
)

// BuildRouter builds the router.
func BuildRouter(c *services.Container) error {
	// Non-static file route group.
	g := c.Web.Group("")

	g.Use(
		echomw.RemoveTrailingSlashWithConfig(echomw.TrailingSlashConfig{
			RedirectCode: http.StatusMovedPermanently,
		}),
		echomw.Recover(),
		echomw.RequestIDWithConfig(echomw.RequestIDConfig{
			RequestIDHandler: middleware.RequestIDHandler,
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
