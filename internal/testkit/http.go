package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/go-rest-template/internal/service"
	"github.com/liukeshao/go-rest-template/pkg/api"
	"github.com/liukeshao/go-rest-template/pkg/httpkit"
)

type reqConfig struct {
	body        io.Reader
	contentType string
	headers     http.Header
}

type ReqOption func(t testing.TB, cfg *reqConfig)

func WithJSON(v any) ReqOption {
	return func(t testing.TB, cfg *reqConfig) {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("testkit.WithJSON marshal failed: %v", err)
		}
		cfg.body = bytes.NewReader(b)
		cfg.contentType = httpkit.ContentTypeJSON
	}
}

func WithBody(contentType, body string) ReqOption {
	return func(_ testing.TB, cfg *reqConfig) {
		cfg.body = strings.NewReader(body)
		cfg.contentType = contentType
	}
}

func WithRawBody(contentType string, body io.Reader) ReqOption {
	return func(_ testing.TB, cfg *reqConfig) {
		cfg.body = body
		cfg.contentType = contentType
	}
}

func WithBearer(accessToken string) ReqOption {
	return WithHeader("Authorization", "Bearer "+accessToken)
}

func WithHeader(key, value string) ReqOption {
	return func(_ testing.TB, cfg *reqConfig) {
		if cfg.headers == nil {
			cfg.headers = make(http.Header)
		}
		cfg.headers.Set(key, value)
	}
}

func (e *Env) DoRequest(method, target string, opts ...ReqOption) *httptest.ResponseRecorder {
	e.t.Helper()

	var cfg reqConfig
	for _, opt := range opts {
		opt(e.t, &cfg)
	}

	req := httptest.NewRequest(method, target, cfg.body)
	if cfg.contentType != "" {
		req.Header.Set("Content-Type", cfg.contentType)
	}
	for k, vals := range cfg.headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	rec := httptest.NewRecorder()
	e.Handler().ServeHTTP(rec, req)
	return rec
}

func (e *Env) MustAuthUser(username, email, password string) *service.TokenPair {
	e.t.Helper()
	authSvc := do.MustInvoke[*service.Auth](e.Injector)
	resp, err := authSvc.Register(context.Background(), &api.RegisterRequest{
		Username: username,
		Email:    email,
		Password: password,
	})
	require.NoError(e.t, err)
	return resp
}
