package middleware

import (
	"github.com/labstack/echo/v5"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/i18n"
)

// Locale 读取请求偏好语言，将协商后的 language.Tag 写入 request context。
// 协商优先级：?lang= 查询参数 > Accept-Language header > Translator.Default()。
//
// 必须放置在 RequestContext 之后，业务 handler 之前。
// 与 error_handler 配合，实现 RFC 9457 Problem.Detail 与字段错误的本地化输出。
func Locale(t i18n.Translator) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			tag := t.Match(
				c.Request().Header.Get("Accept-Language"),
				c.QueryParam("lang"),
			)
			ctx := appctx.WithLocale(c.Request().Context(), tag)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
