package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) rest.Problem {
	t.Helper()
	assert.Contains(t, rec.Header().Get("Content-Type"), rest.ContentTypeProblemJSON)
	var p rest.Problem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p), "body: %s", rec.Body.String())
	return p
}

func TestDefaultErrorEncoder_AllPaths(t *testing.T) {
	t.Parallel()
	router := rest.NewRouter()
	router.Use(rest.AccessLog(), rest.Recover(nil), rest.BodyLimit(64))

	router.HandleFunc("GET /problem", func(w http.ResponseWriter, r *http.Request) {
		rest.DefaultErrorEncoder(w, r, rest.Problem{
			Status: http.StatusForbidden,
			Code:   "POLICY_DENIED",
			Detail: "Forbidden by policy.",
		})
	})
	router.HandleFunc("GET /validation", func(w http.ResponseWriter, r *http.Request) {
		var v validator.FieldViolations
		v.Add("title", validator.CodeRequired, nil)
		rest.DefaultErrorEncoder(w, r, v.Err())
	})
	router.HandleFunc("GET /deadline", func(w http.ResponseWriter, r *http.Request) {
		rest.DefaultErrorEncoder(w, r, context.DeadlineExceeded)
	})
	router.HandleFunc("GET /canceled", func(w http.ResponseWriter, r *http.Request) {
		rest.DefaultErrorEncoder(w, r, context.Canceled)
	})
	router.HandleFunc("GET /unknown", func(w http.ResponseWriter, r *http.Request) {
		rest.DefaultErrorEncoder(w, r, errors.New("secret connection string"))
	})
	router.HandleFunc("GET /panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})
	router.HandleFunc("POST /decode", func(w http.ResponseWriter, r *http.Request) {
		var dst struct {
			Name string `json:"name"`
		}
		if err := rest.DecodeJSON(r, &dst); err != nil {
			rest.DefaultErrorEncoder(w, r, err)
			return
		}
		rest.WriteJSON(w, r, http.StatusOK, dst)
	})

	call := func(method, target, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	rec := call(http.MethodGet, "/problem", "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	p := decodeProblem(t, rec)
	assert.Equal(t, rest.Code("POLICY_DENIED"), p.Code)
	assert.Equal(t, "/problem", p.Instance)

	rec = call(http.MethodGet, "/validation", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	p = decodeProblem(t, rec)
	assert.Equal(t, rest.CodeBadRequest, p.Code)
	valMeta, err := rest.DecodeMetadata[rest.ValidationInfo](p)
	require.NoError(t, err)
	assert.Equal(t, 1, valMeta.ErrorCount)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "title", p.Errors[0].Field)
	assert.Equal(t, validator.CodeRequired, p.Errors[0].Code)

	rec = call(http.MethodGet, "/deadline", "")
	assert.Equal(t, http.StatusGatewayTimeout, rec.Code)
	assert.Equal(t, rest.CodeGatewayTimeout, decodeProblem(t, rec).Code)

	rec = call(http.MethodGet, "/canceled", "")
	assert.Equal(t, rest.StatusClientClosedRequest, rec.Code)
	assert.Equal(t, rest.CodeClientClosedRequest, decodeProblem(t, rec).Code)

	rec = call(http.MethodGet, "/unknown", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	p = decodeProblem(t, rec)
	assert.Equal(t, rest.CodeInternal, p.Code)
	assert.NotContains(t, p.Detail, "secret")

	rec = call(http.MethodGet, "/panic", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, rest.CodeInternal, decodeProblem(t, rec).Code)

	// Trailing JSON garbage rejected as 400 BAD_REQUEST with DecodeInfo metadata.
	rec = call(http.MethodPost, "/decode", `{"name":"a"}{"name":"b"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	p = decodeProblem(t, rec)
	assert.Equal(t, rest.CodeBadRequest, p.Code)
	decMeta, err := rest.DecodeMetadata[rest.DecodeInfo](p)
	require.NoError(t, err)
	assert.Equal(t, rest.DecodeReasonTrailingData, decMeta.Reason)

	// Empty body rejected as 400 BAD_REQUEST with DecodeInfo(empty_body).
	rec = call(http.MethodPost, "/decode", `   `)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	p = decodeProblem(t, rec)
	decMeta, err = rest.DecodeMetadata[rest.DecodeInfo](p)
	require.NoError(t, err)
	assert.Equal(t, rest.DecodeReasonEmptyBody, decMeta.Reason)

	// Oversized body rejected as 413 CONTENT_TOO_LARGE with RequestLimitInfo.
	rec = call(http.MethodPost, "/decode", `{"name":"`+strings.Repeat("x", 128)+`"}`)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	p = decodeProblem(t, rec)
	assert.Equal(t, rest.CodeContentTooLarge, p.Code)
	limitMeta, err := rest.DecodeMetadata[rest.RequestLimitInfo](p)
	require.NoError(t, err)
	assert.Equal(t, int64(64), limitMeta.MaxBytes)

	// 404 & 405 converted to RFC 9457 Problem.
	rec = call(http.MethodGet, "/missing", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, rest.CodeNotFound, decodeProblem(t, rec).Code)

	rec = call(http.MethodPost, "/problem", "")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, rest.CodeMethodNotAllowed, decodeProblem(t, rec).Code)
	assert.Contains(t, rec.Header().Get("Allow"), "GET")

	// Path-cleaning redirect from ServeMux is preserved (not converted to 404).
	rec = call(http.MethodGet, "/foo/../problem", "")
	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	assert.Equal(t, "/problem", rec.Header().Get("Location"))
}

func TestHandler_StandaloneAndMounted(t *testing.T) {
	t.Parallel()

	h := rest.NewHandler(func(mux *http.ServeMux) {
		mux.HandleFunc("GET /ping/{name}", func(w http.ResponseWriter, r *http.Request) {
			rest.WriteJSON(w, r, http.StatusOK, map[string]string{"hello": r.PathValue("name")})
		})
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping/world", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"hello":"world"}`, rec.Body.String())

	router := rest.NewRouter()
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

	// 1. OR: BearerAuth | ApiKeyAuth.
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

	// 2. AND: [BearerAuth, ApiKeyAuth].
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
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, rest.Code("API_KEY_INVALID"), decodeProblem(t, rec).Code)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ok")
	req.Header.Set("X-API-Key", "secret")
	rec = httptest.NewRecorder()
	andHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 3. Optional auth: BearerAuth | NoAuth.
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

func TestProblem_ErrorStringAndCodeSentinel(t *testing.T) {
	t.Parallel()
	p := rest.Problem{
		Status: http.StatusNotFound,
		Code:   rest.CodeNotFound,
		Detail: "Book 01TEST was not found.",
	}
	assert.Equal(t, "http 404 NOT_FOUND: Book 01TEST was not found.", p.Error())
	assert.Equal(t, http.StatusNotFound, rest.CodeNotFound.HTTPStatus())
	assert.Equal(t, http.StatusConflict, rest.CodeConflict.HTTPStatus())
	assert.Equal(t, http.StatusInternalServerError, rest.Code("UNKNOWN").HTTPStatus())
	assert.Len(t, rest.AllCodes(), 13)
	assert.Equal(t, "NOT_FOUND", rest.CodeNotFound.Error())
}
