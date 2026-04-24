package handlers

import (
	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/middleware"
)

// BuildRouter builds the router.
func BuildRouter(i do.Injector, web *echo.Echo) error {
	cfg := do.MustInvoke[*config.Config](i)
	translator := resolveTranslator(i)

	registerErrorHandler(web, translator)

	g := web.Group("")
	g.Use(
		echomw.Recover(),
		echomw.RequestID(),
		middleware.RequestContext(),   // writes X-Request-ID into ctx for slog
		middleware.Locale(translator), // negotiates Accept-Language → ctx
		echomw.RequestLogger(),
	)

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
