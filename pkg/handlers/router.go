package handlers

import (
	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/i18n"
	"github.com/liukeshao/echo-template/pkg/middleware"
)

const defaultMaxBodyBytes = 2 << 20 // 2 MiB

// BuildRouter builds the router.
func BuildRouter(i do.Injector, web *echo.Echo) error {
	cfg := do.MustInvoke[*config.Config](i)
	translator := do.MustInvoke[i18n.Translator](i)

	registerErrorHandler(web, translator)

	// 注册到根 Echo 实例（而非 Group），确保 404/405 等未命中路由也执行
	// Recover、RequestID、RequestContext、Locale、BodyLimit 与 RequestLogger。
	web.Use(
		echomw.Recover(),
		echomw.RequestID(),
		middleware.RequestContext(),   // writes X-Request-ID into ctx for slog
		middleware.Locale(translator), // negotiates Accept-Language → ctx
		echomw.BodyLimit(defaultMaxBodyBytes),
		echomw.RequestLogger(),
	)

	g := web.Group("")

	for _, h := range []Handler{
		do.MustInvoke[*AuthHandler](i),
		do.MustInvoke[*BookHandler](i),
		do.MustInvoke[*MeHandler](i),
	} {
		h.Routes(g)
	}

	// API docs are not exposed in production.
	if cfg.App.Environment != config.EnvProduction {
		do.MustInvoke[*DocsHandler](i).Routes(g)
	}

	return nil
}
