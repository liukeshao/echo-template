package server

import (
	"context"
	"net/http"
	"time"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/go-rest-template/config"
	"github.com/liukeshao/go-rest-template/internal/auth"
	"github.com/liukeshao/go-rest-template/internal/database"
	"github.com/liukeshao/go-rest-template/internal/handler"
	"github.com/liukeshao/go-rest-template/internal/i18n"
	"github.com/liukeshao/go-rest-template/pkg/api"
	"github.com/liukeshao/go-rest-template/pkg/httperr"
	"github.com/liukeshao/go-rest-template/pkg/httpkit"
	"github.com/liukeshao/go-rest-template/static"
)

const (
	defaultMaxBodyBytes = 2 << 20
	readinessTimeout    = 2 * time.Second
)

var errHealth = oops.In("health").Tags("database")

func BuildRouter(i do.Injector) (http.Handler, error) {
	cfg := do.MustInvoke[*config.Config](i)
	tr := do.MustInvoke[i18n.Translator](i)
	jwtMgr := do.MustInvoke[*auth.JWT](i)

	encodeErr := NewProblemEncoder(tr)

	maxBodyBytes := cfg.HTTP.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}

	opts := []httpkit.HandlerOption{
		httpkit.WithErrorEncoder(encodeErr),
		httpkit.WithAuthScheme("BearerAuth", auth.BearerAuthenticator(jwtMgr)),
	}

	router := httpkit.NewRouter()
	router.SetErrorEncoder(encodeErr)
	router.Use(
		httpkit.RequestID(),
		Locale(tr),
		httpkit.CORS(httpkit.CORSConfig{
			AllowedOrigins:   cfg.HTTP.CORS.AllowedOrigins,
			AllowedMethods:   cfg.HTTP.CORS.AllowedMethods,
			AllowedHeaders:   cfg.HTTP.CORS.AllowedHeaders,
			ExposedHeaders:   cfg.HTTP.CORS.ExposedHeaders,
			AllowCredentials: cfg.HTTP.CORS.AllowCredentials,
			MaxAge:           cfg.HTTP.CORS.MaxAge,
		}),
		httpkit.AccessLog("/healthz", "/readyz"),
		httpkit.Recover(encodeErr),
		httpkit.BodyLimit(maxBodyBytes),
	)

	router.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpkit.WriteJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		db, err := do.Invoke[*database.DB](i)
		if err != nil {
			encodeErr(w, r, errHealth.With("op", "invoking database").Join(httperr.ServiceUnavailable, err))
			return
		}
		pingCtx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()
		if err := db.PingContext(pingCtx); err != nil {
			encodeErr(w, r, errHealth.With("op", "pinging database").Join(httperr.ServiceUnavailable, err))
			return
		}
		httpkit.WriteJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})

	router.Mount(
		api.NewAuthHandler(do.MustInvoke[*handler.Auth](i), opts...),
		api.NewBooksHandler(do.MustInvoke[*handler.Books](i), opts...),
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
