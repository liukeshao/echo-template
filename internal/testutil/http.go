package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/pkg/api"
)

const mimeApplicationJSON = "application/json"

type reqConfig struct {
	body        io.Reader
	contentType string
	headers     http.Header
}

// ReqOption 自定义测试 HTTP 请求的请求体与请求头。
type ReqOption func(t interface{ Helper(); Fatalf(string, ...any) }, cfg *reqConfig)

// WithJSON 将任意值序列化为 JSON 请求体并自动设置 Content-Type: application/json。
func WithJSON(v any) ReqOption {
	return func(t interface{ Helper(); Fatalf(string, ...any) }, cfg *reqConfig) {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("testutil.WithJSON marshal failed: %v", err)
		}
		cfg.body = bytes.NewReader(b)
		cfg.contentType = mimeApplicationJSON
	}
}

// WithBody 设置原始字符串请求体和指定的 Content-Type（适合 PATCH / 畸形 JSON 测试）。
func WithBody(contentType, body string) ReqOption {
	return func(_ interface{ Helper(); Fatalf(string, ...any) }, cfg *reqConfig) {
		cfg.body = strings.NewReader(body)
		cfg.contentType = contentType
	}
}

// WithRawBody 设置原始 io.Reader 请求体和指定的 Content-Type（适合大请求体等流式测试）。
func WithRawBody(contentType string, body io.Reader) ReqOption {
	return func(_ interface{ Helper(); Fatalf(string, ...any) }, cfg *reqConfig) {
		cfg.body = body
		cfg.contentType = contentType
	}
}

// WithBearer 设置 Authorization: Bearer <token> 请求头。
func WithBearer(accessToken string) ReqOption {
	return WithHeader("Authorization", "Bearer "+accessToken)
}

// WithHeader 添加自定义请求头（例如 Accept-Language）。
func WithHeader(key, value string) ReqOption {
	return func(_ interface{ Helper(); Fatalf(string, ...any) }, cfg *reqConfig) {
		if cfg.headers == nil {
			cfg.headers = make(http.Header)
		}
		cfg.headers.Set(key, value)
	}
}

// DoRequest 向测试 HTTP Handler 发起一次内存 HTTP 请求并返回响应记录器。
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

// MustAuthUser 注册一个测试用户并返回其认证令牌，用于快速构造需要登录态的测试用例。
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
