package middleware

import (
	"github.com/labstack/echo/v4"
	appContext "github.com/liukeshao/echo-template/pkg/context"
)

// RequestIDHandler 将 request ID 注入到 context 中
func RequestIDHandler(c echo.Context, requestID string) {
	// 使用新的 WithRequestID 方法设置 request ID
	ctx := appContext.WithRequestID(c.Request().Context(), requestID)
	c.SetRequest(c.Request().WithContext(ctx))
}
