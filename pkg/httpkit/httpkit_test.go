package httpkit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/httpkit"
	"github.com/liukeshao/echo-template/pkg/logging"
)

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) httperr.Problem {
	t.Helper()
	assert.Contains(t, rec.Header().Get("Content-Type"), httperr.ContentTypeProblemJSON)
	var p httperr.Problem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p), "body: %s", rec.Body.String())
	return p
}

func TestRouter_MiddlewareDecodeAndFallbacks(t *testing.T) {
	t.Parallel()
	router := httpkit.NewRouter()
	router.Use(httpkit.RequestID(), httpkit.AccessLog(), httpkit.Recover(nil), httpkit.BodyLimit(64))

	router.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		httpkit.WriteJSON(w, r, http.StatusOK, map[string]string{
			"status":     "ok",
			"request_id": logging.RequestID(r.Context()),
		})
	})
	router.HandleFunc("GET /panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})
	router.HandleFunc("POST /decode", func(w http.ResponseWriter, r *http.Request) {
		var dst struct {
			Name string `json:"name"`
		}
		if err := httpkit.DecodeJSON(r, &dst); err != nil {
			httperr.DefaultEncoder(w, r, err)
			return
		}
		httpkit.WriteJSON(w, r, http.StatusOK, dst)
	})

	call := func(method, target, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// RequestID generates UUID v7 when absent and preserves incoming X-Request-ID header.
	rec := call(http.MethodGet, "/ok", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(httpkit.HeaderRequestID))
	reqWithID := httptest.NewRequest(http.MethodGet, "/ok", nil)
	reqWithID.Header.Set(httpkit.HeaderRequestID, "rid-123")
	recWithID := httptest.NewRecorder()
	router.ServeHTTP(recWithID, reqWithID)
	assert.Equal(t, "rid-123", recWithID.Header().Get(httpkit.HeaderRequestID))
	assert.Contains(t, recWithID.Body.String(), `"request_id":"rid-123"`)

	// Panic recovered -> 500 RFC 9457 Problem.
	rec = call(http.MethodGet, "/panic", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, http.StatusInternalServerError, decodeProblem(t, rec).Status)

	// Trailing JSON garbage rejected as 400 with DecodeInfo metadata.
	rec = call(http.MethodPost, "/decode", `{"name":"a"}{"name":"b"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	p := decodeProblem(t, rec)
	decMeta, err := httperr.DecodeMetadata[httperr.DecodeInfo](p)
	require.NoError(t, err)
	assert.Equal(t, httperr.DecodeReasonTrailingData, decMeta.Reason)

	// Empty body rejected as 400 with DecodeInfo(empty_body).
	rec = call(http.MethodPost, "/decode", `   `)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	p = decodeProblem(t, rec)
	decMeta, err = httperr.DecodeMetadata[httperr.DecodeInfo](p)
	require.NoError(t, err)
	assert.Equal(t, httperr.DecodeReasonEmptyBody, decMeta.Reason)

	// Oversized body rejected as 413 with RequestLimitInfo.
	rec = call(http.MethodPost, "/decode", `{"name":"`+strings.Repeat("x", 128)+`"}`)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	p = decodeProblem(t, rec)
	limitMeta, err := httperr.DecodeMetadata[httperr.RequestLimitInfo](p)
	require.NoError(t, err)
	assert.Equal(t, int64(64), limitMeta.MaxBytes)

	// 404 & 405 converted to RFC 9457 Problem.
	rec = call(http.MethodGet, "/missing", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, http.StatusNotFound, decodeProblem(t, rec).Status)

	rec = call(http.MethodPost, "/ok", "")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, http.StatusMethodNotAllowed, decodeProblem(t, rec).Status)
	assert.Contains(t, rec.Header().Get("Allow"), "GET")

	// Path-cleaning redirect from ServeMux is preserved (not converted to 404).
	rec = call(http.MethodGet, "/foo/../ok", "")
	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	assert.Equal(t, "/ok", rec.Header().Get("Location"))
}

func TestHandler_StandaloneAndMounted(t *testing.T) {
	t.Parallel()

	h := httpkit.NewHandler(func(mux *http.ServeMux) {
		mux.HandleFunc("GET /ping/{name}", func(w http.ResponseWriter, r *http.Request) {
			httpkit.WriteJSON(w, r, http.StatusOK, map[string]string{"hello": r.PathValue("name")})
		})
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping/world", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"hello":"world"}`, rec.Body.String())

	router := httpkit.NewRouter()
	router.Mount(h)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping/router", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"hello":"router"}`, rec.Body.String())
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
			return ctx, httperr.Problem{Status: http.StatusUnauthorized, Metadata: httperr.PreconditionInfo{Type: "BEARER_INVALID"}}
		}
		return context.WithValue(ctx, bearerKey, "u1"), nil
	}
	apiKey := func(ctx context.Context, r *http.Request) (context.Context, error) {
		if r.Header.Get("X-API-Key") != "secret" {
			return ctx, httperr.Problem{Status: http.StatusUnauthorized, Metadata: httperr.PreconditionInfo{Type: "API_KEY_INVALID"}}
		}
		return context.WithValue(ctx, apiKeyKey, "k1"), nil
	}

	var trace []string
	cfg := httpkit.ApplyHandlerOptions(
		httpkit.WithAuthScheme("BearerAuth", bearer),
		httpkit.WithAuthScheme("ApiKeyAuth", apiKey),
		httpkit.WithMiddleware(func(op httpkit.Operation) httpkit.Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					trace = append(trace, op.ID)
					next.ServeHTTP(w, r)
				})
			}
		}),
	)

	// 1. OR: BearerAuth | ApiKeyAuth.
	orHandler := cfg.Wrap(httpkit.Operation{
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

	// 2. AND: [BearerAuth, ApiKeyAuth].
	andHandler := cfg.Wrap(httpkit.Operation{
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
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	meta, err := httperr.DecodeMetadata[httperr.PreconditionInfo](decodeProblem(t, rec))
	require.NoError(t, err)
	assert.Equal(t, "API_KEY_INVALID", meta.Type)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ok")
	req.Header.Set("X-API-Key", "secret")
	rec = httptest.NewRecorder()
	andHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 3. Optional auth: BearerAuth | NoAuth.
	optHandler := cfg.Wrap(httpkit.Operation{
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
		cfg.Wrap(httpkit.Operation{
			ID:   "missingScheme",
			Auth: [][]string{{"OAuth2Auth"}},
		}, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	// 5. Third-party net/http auth middleware works directly via WithMiddleware without WithAuthScheme.
	customCfg := httpkit.ApplyHandlerOptions(
		httpkit.WithMiddleware(func(op httpkit.Operation) httpkit.Middleware {
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
	customCfg.Wrap(httpkit.Operation{
		ID:   "customProtected",
		Auth: [][]string{{"CustomAuth"}},
	}, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "customProtected", rec.Header().Get("X-Custom-Auth"))
}

func TestServer_ShutdownBeforeStartAndStartAfterShutdown(t *testing.T) {
	t.Parallel()
	srv := httpkit.NewServer(httpkit.ServerConfig{
		Addr:            "127.0.0.1:0",
		GracefulTimeout: time.Second,
	}, http.NewServeMux())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx))
	require.NoError(t, srv.Start())
}
