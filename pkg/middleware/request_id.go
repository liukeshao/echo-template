package middleware

import (
	"context"

	"github.com/labstack/echo/v4"
	appContext "github.com/liukeshao/echo-template/pkg/context"
)

// requestIDHandler injects the request ID into the standard context
func RequestIDHandler(c echo.Context, requestID string) {
	// 将 request ID 注入到标准包的 context 中
	ctx := context.WithValue(c.Request().Context(), appContext.RequestIDKey, requestID)
	c.SetRequest(c.Request().WithContext(ctx))
}
