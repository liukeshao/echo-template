package server_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/internal/testutil"
)

func TestDocs_StaticIsolatesAssets(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)

	// /docs, /openapi.yaml, and embedded JS/CSS assets are served.
	for _, p := range []string{"/docs", "/openapi.yaml", "/static/js/web-components.min.js", "/static/css/styles.min.css"} {
		rec := env.DoRequest(http.MethodGet, p)
		assert.Equal(t, http.StatusOK, rec.Code, "path=%s", p)
	}

	// /static/* must not expose index.html, openapi.yaml, or directory listings.
	for _, p := range []string{"/static/", "/static/js/", "/static/index.html", "/static/openapi/openapi.yaml"} {
		rec := env.DoRequest(http.MethodGet, p)
		assert.Equal(t, http.StatusNotFound, rec.Code, "path=%s", p)
	}
}

func TestDocs_DisabledInProduction(t *testing.T) {
	t.Parallel()
	env := testutil.New(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.App.Environment = config.EnvProduction
	}))

	for _, p := range []string{"/docs", "/openapi.yaml", "/static/js/web-components.min.js"} {
		rec := env.DoRequest(http.MethodGet, p)
		assert.Equal(t, http.StatusNotFound, rec.Code, "path=%s must be disabled in prod", p)
	}
}
