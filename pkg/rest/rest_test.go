package rest_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/samber/do/v2"
	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/i18n"
	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/testutil"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestRouter_WriteProblemAllClassificationPaths(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	tr := do.MustInvoke[i18n.Translator](env.Injector)
	rt := rest.New(tr)

	router := rest.NewRouter(rt, rest.WithMaxBodyBytes(1<<20))
	router.HandleFunc("GET /test/problem", func(w http.ResponseWriter, r *http.Request) {
		rt.WriteProblem(w, r, rest.Problem{
			Status: http.StatusForbidden,
			Code:   "POLICY_DENIED",
			Detail: "Forbidden by policy.",
		})
	})
	router.HandleFunc("GET /test/oops-localized", func(w http.ResponseWriter, r *http.Request) {
		rt.WriteProblem(w, r, oops.
			Code(codes.BookNotFound.String()).
			With(codes.CtxKeyParams, map[string]string{"book_id": "01TEST"}).
			Public("Book not found").
			Errorf("book not found"))
	})
	router.HandleFunc("GET /test/oops-internal", func(w http.ResponseWriter, r *http.Request) {
		rt.WriteProblem(w, r, oops.
			Code(codes.BookInternal.String()).
			With(codes.CtxKeyParams, map[string]string{"sql": "SELECT secret"}).
			Wrapf(errors.New("db password leak"), "querying book"))
	})
	router.HandleFunc("GET /test/validation", func(w http.ResponseWriter, r *http.Request) {
		var v validator.FieldViolations
		v.Add("title", validator.KeyRequired, map[string]string{"field": "title"})
		rt.WriteProblem(w, r, v.Err())
	})
	router.HandleFunc("GET /test/deadline", func(w http.ResponseWriter, r *http.Request) {
		rt.WriteProblem(w, r, context.DeadlineExceeded)
	})
	router.HandleFunc("GET /test/canceled", func(w http.ResponseWriter, r *http.Request) {
		rt.WriteProblem(w, r, context.Canceled)
	})
	router.HandleFunc("GET /test/unknown", func(w http.ResponseWriter, r *http.Request) {
		rt.WriteProblem(w, r, errors.New("secret database connection string"))
	})
	router.HandleFunc("GET /test/panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("unexpected crash")
	})

	call := func(method, target, lang string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		if lang != "" {
			req.Header.Set("Accept-Language", lang)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// 1. Direct Problem -> fills type, title, instance, request_id
	rec := call(http.MethodGet, "/test/problem", "")
	p := testutil.AssertProblem(t, rec, http.StatusForbidden, "POLICY_DENIED")
	assert.Equal(t, "about:blank", p.Type)
	assert.Equal(t, "Forbidden", p.Title)
	assert.Equal(t, "Forbidden by policy.", p.Detail)
	assert.Equal(t, "/test/problem", p.Instance)
	assert.Equal(t, rec.Header().Get(rest.HeaderRequestID), p.RequestID)

	// 2. oops with CtxKeyParams -> localized Chinese detail + exposed params
	rec = call(http.MethodGet, "/test/oops-localized", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusNotFound, "BOOK_NOT_FOUND")
	assert.Equal(t, "未找到 ID 为 01TEST 的书籍。", p.Detail)
	assert.Equal(t, map[string]string{"book_id": "01TEST"}, p.Params)

	// 3. oops 5xx -> localized generic message, internal error & params hidden
	rec = call(http.MethodGet, "/test/oops-internal", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusInternalServerError, "BOOK_INTERNAL_ERROR")
	assert.Equal(t, "处理书籍请求时发生内部错误。", p.Detail)
	assert.NotContains(t, p.Detail, "db password leak")
	assert.Empty(t, p.Params)

	// 4. *validator.FieldViolations -> localized 400 + errors
	rec = call(http.MethodGet, "/test/validation", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_INVALID")
	assert.Equal(t, "请求参数校验失败。", p.Detail)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "title", p.Errors[0].Field)
	assert.Equal(t, "title 不能为空。", p.Errors[0].Message)

	// 5. context.DeadlineExceeded / context.Canceled (localized in zh-CN)
	p = testutil.AssertProblem(t, call(http.MethodGet, "/test/deadline", "zh-CN"), http.StatusGatewayTimeout, "GATEWAY_TIMEOUT")
	assert.Equal(t, "请求处理超时。", p.Detail)
	p = testutil.AssertProblem(t, call(http.MethodGet, "/test/canceled", "zh-CN"), 499, "CLIENT_CLOSED_REQUEST")
	assert.Equal(t, "客户端在服务端响应前关闭了连接。", p.Detail)

	// 6. Unknown error -> 500 with details hidden
	p = testutil.AssertProblem(t, call(http.MethodGet, "/test/unknown", ""), http.StatusInternalServerError, "INTERNAL_ERROR")
	assert.NotContains(t, p.Detail, "secret database")

	// 7. Panic recovered -> 500 RFC 9457 Problem (preserving RequestID and negotiated locale)
	rec = call(http.MethodGet, "/test/panic", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusInternalServerError, "INTERNAL_ERROR")
	assert.NotContains(t, p.Detail, "unexpected crash")
	assert.Equal(t, "服务器内部错误。", p.Detail)
	assert.NotEmpty(t, p.RequestID)
	assert.Equal(t, rec.Header().Get(rest.HeaderRequestID), p.RequestID)

	// 8. 404 & 405 from ServeMux converted to localized RFC 9457 Problem
	p = testutil.AssertProblem(t, call(http.MethodGet, "/does-not-exist", "zh-CN"), http.StatusNotFound, "NOT_FOUND")
	assert.Equal(t, "/does-not-exist", p.Instance)
	assert.Equal(t, "请求的资源不存在。", p.Detail)

	rec = call(http.MethodPost, "/test/problem", "zh-CN")
	p = testutil.AssertProblem(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
	assert.Equal(t, "该资源不支持当前 HTTP 方法。", p.Detail)
	assert.Contains(t, rec.Header().Get("Allow"), "GET")
}

func TestHandler_StandaloneAndMounted(t *testing.T) {
	t.Parallel()

	// Standalone rest.Handler works directly as a standard http.Handler (Connect-Go style).
	h := rest.NewHandler(func(mux *http.ServeMux) {
		mux.HandleFunc("GET /ping/{name}", func(w http.ResponseWriter, r *http.Request) {
			rest.DefaultRuntime().WriteJSON(w, http.StatusOK, map[string]string{"hello": r.PathValue("name")})
		})
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping/world", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"hello":"world"}`, rec.Body.String())

	// Same rest.Handler mounts cleanly onto rest.Router.
	router := rest.NewRouter(nil)
	router.Mount(h)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping/router", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"hello":"router"}`, rec.Body.String())
}

func TestMiddlewares_RequestIDAndLocale(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	tr := do.MustInvoke[i18n.Translator](env.Injector)
	rt := rest.New(tr)

	var gotRID, gotLocale string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRID = appctx.RequestIDFromContext(r.Context())
		gotLocale = appctx.LocaleFromContext(r.Context(), tr.Default()).String()
		w.WriteHeader(http.StatusOK)
	})

	h := rest.RequestID()(rt.Locale()(inner))
	req := httptest.NewRequest(http.MethodGet, "/?lang=zh-CN", nil)
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set(rest.HeaderRequestID, "rid-999")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "rid-999", gotRID)
	assert.Equal(t, "rid-999", rec.Header().Get(rest.HeaderRequestID))
	assert.Equal(t, "zh-CN", gotLocale)
}

func TestServer_ShutdownBeforeStartDoesNotDeadlock(t *testing.T) {
	t.Parallel()
	cfg := testutil.DefaultConfig()
	srv := rest.NewServer(&cfg, http.NewServeMux())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx))
}

func TestHandlerConfig_AuthSchemesAndRouteMiddleware(t *testing.T) {
	t.Parallel()

	type ctxKey string
	const (
		bearerKey ctxKey = "bearer"
		apiKeyKey ctxKey = "api_key"
	)

	bearer := func(ctx context.Context, r *http.Request) (context.Context, error) {
		if r.Header.Get("Authorization") != "Bearer ok" {
			return ctx, rest.Problem{Status: http.StatusUnauthorized, Code: "BEARER_INVALID", Detail: "bad bearer"}
		}
		return context.WithValue(ctx, bearerKey, "u1"), nil
	}
	apiKey := func(ctx context.Context, r *http.Request) (context.Context, error) {
		if r.Header.Get("X-API-Key") != "secret" {
			return ctx, rest.Problem{Status: http.StatusUnauthorized, Code: "API_KEY_INVALID", Detail: "bad api key"}
		}
		return context.WithValue(ctx, apiKeyKey, "k1"), nil
	}

	var trace []string
	cfg := rest.ApplyHandlerOptions(
		rest.WithAuthScheme("BearerAuth", bearer),
		rest.WithAuthScheme("ApiKeyAuth", apiKey),
		rest.WithMiddleware(func(op rest.Operation) rest.Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					trace = append(trace, op.ID)
					next.ServeHTTP(w, r)
				})
			}
		}),
	)

	// 1. OR: BearerAuth | ApiKeyAuth (second scheme succeeds when first fails).
	orHandler := cfg.Wrap(rest.Operation{
		ID:   "dualAuth",
		Auth: [][]string{{"BearerAuth"}, {"ApiKeyAuth"}},
	}, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "k1", r.Context().Value(apiKeyKey))
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()
	orHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 2. AND: [BearerAuth, ApiKeyAuth] requires both.
	andHandler := cfg.Wrap(rest.Operation{
		ID:   "bothAuth",
		Auth: [][]string{{"BearerAuth", "ApiKeyAuth"}},
	}, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "u1", r.Context().Value(bearerKey))
		assert.Equal(t, "k1", r.Context().Value(apiKeyKey))
		w.WriteHeader(http.StatusOK)
	})
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ok")
	rec = httptest.NewRecorder()
	andHandler.ServeHTTP(rec, req)
	testutil.AssertProblem(t, rec, http.StatusUnauthorized, "API_KEY_INVALID")

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ok")
	req.Header.Set("X-API-Key", "secret")
	rec = httptest.NewRecorder()
	andHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 3. Optional auth: BearerAuth | NoAuth allows anonymous pass-through.
	optHandler := cfg.Wrap(rest.Operation{
		ID:   "optAuth",
		Auth: [][]string{{"BearerAuth"}, {}},
	}, func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, r.Context().Value(bearerKey))
		w.WriteHeader(http.StatusOK)
	})
	rec = httptest.NewRecorder()
	optHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"dualAuth", "bothAuth", "optAuth"}, trace)

	// 4. Unregistered scheme fails fast with panic at mount time.
	assert.Panics(t, func() {
		cfg.Wrap(rest.Operation{
			ID:   "missingScheme",
			Auth: [][]string{{"OAuth2Auth"}},
		}, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	// 5. Third-party net/http auth middleware works directly via WithMiddleware without WithAuthScheme.
	customCfg := rest.ApplyHandlerOptions(
		rest.WithMiddleware(func(op rest.Operation) rest.Middleware {
			if !op.HasAuth() {
				return nil
			}
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Custom-Auth", op.ID)
					next.ServeHTTP(w, r)
				})
			}
		}),
	)
	rec = httptest.NewRecorder()
	customCfg.Wrap(rest.Operation{
		ID:   "customProtected",
		Auth: [][]string{{"CustomAuth"}},
	}, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "customProtected", rec.Header().Get("X-Custom-Auth"))
}

func TestProblem_ErrorString(t *testing.T) {
	t.Parallel()
	p := rest.Problem{
		Status: http.StatusNotFound,
		Code:   "BOOK_NOT_FOUND",
		Detail: "Book 01TEST was not found.",
	}
	assert.Equal(t, "http 404 BOOK_NOT_FOUND: Book 01TEST was not found.", p.Error())
}
