package server_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/httperr"
)

func TestDocs_StaticIsolatesAssets(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)

	// /docs, /openapi.yaml, and embedded JS/CSS assets are served with expected Content-Type.
	cases := []struct {
		path        string
		contentType string
	}{
		{"/docs", "text/html"},
		{"/openapi.yaml", "application/yaml"},
		{"/static/js/web-components.min.js", "javascript"},
		{"/static/css/styles.min.css", "text/css"},
	}
	for _, tc := range cases {
		rec := env.DoRequest(http.MethodGet, tc.path)
		assert.Equal(t, http.StatusOK, rec.Code, "path=%s", tc.path)
		assert.Contains(t, rec.Header().Get("Content-Type"), tc.contentType, "path=%s", tc.path)
	}

	// /static/* must not expose index.html, openapi.yaml, or directory listings (RFC 9457 404).
	for _, p := range []string{"/static/", "/static/js/", "/static/index.html", "/static/openapi/openapi.yaml"} {
		rec := env.DoRequest(http.MethodGet, p)
		assert.Equal(t, http.StatusNotFound, rec.Code, "path=%s", p)
		assert.Contains(t, rec.Header().Get("Content-Type"), httperr.ContentTypeProblemJSON, "path=%s", p)
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
