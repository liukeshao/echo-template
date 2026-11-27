package server

import (
	"net/http"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/internal/auth"
	"github.com/liukeshao/echo-template/internal/handler"
	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/httpkit"
	"github.com/liukeshao/echo-template/static"
)

const defaultMaxBodyBytes = 2 << 20

func BuildRouter(i do.Injector) (http.Handler, error) {
	cfg := do.MustInvoke[*config.Config](i)
	tr := do.MustInvoke[i18n.Translator](i)
	jwtMgr := do.MustInvoke[*auth.JWT](i)

	encodeErr := NewProblemEncoder(tr)

	opts := []httpkit.HandlerOption{
		httpkit.WithErrorEncoder(encodeErr),
		httpkit.WithAuthScheme("BearerAuth", auth.BearerAuthenticator(jwtMgr)),
	}

	router := httpkit.NewRouter()
	router.SetErrorEncoder(encodeErr)
	router.Use(
		httpkit.RequestID(),
		Locale(tr),
		httpkit.AccessLog(),
		httpkit.Recover(encodeErr),
		httpkit.BodyLimit(defaultMaxBodyBytes),
	)
	router.Mount(
		api.NewAuthHandler(do.MustInvoke[*handler.Auth](i), opts...),
		api.NewBooksHandler(do.MustInvoke[*handler.Book](i), opts...),
		api.NewMeHandler(do.MustInvoke[*handler.Me](i), opts...),
	)

	if cfg.App.Environment != config.EnvProduction {
		router.FileFS("/docs", "index.html", static.DocsFS)
		router.FileFS("/openapi.yaml", "openapi/openapi.yaml", static.DocsFS)
		router.StaticFS("/static", static.AssetsFS)
	}

	return router, nil
}

// Locale 按 ?lang= > Accept-Language > Default 协商语言并写入 context。
func Locale(tr i18n.Translator) httpkit.Middleware {
	if tr == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tag := tr.Match(
				r.Header.Get("Accept-Language"),
				r.URL.Query().Get("lang"),
			)
			next.ServeHTTP(w, r.WithContext(i18n.WithLocale(r.Context(), tag)))
		})
	}
}
