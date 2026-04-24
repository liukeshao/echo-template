package middleware

import (
	"github.com/labstack/echo/v5"

	"github.com/liukeshao/echo-template/pkg/appctx"
)

// RequestContext reads the X-Request-ID set by echomw.RequestID() and writes it
// into the request context so that log.ContextHandler can attach it to every log record.
//
// Must be placed AFTER echomw.RequestID() in the middleware chain.
func RequestContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			rid := c.Response().Header().Get(echo.HeaderXRequestID)
			if rid != "" {
				ctx := appctx.WithRequestID(c.Request().Context(), rid)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	}
}
