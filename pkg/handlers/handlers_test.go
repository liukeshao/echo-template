package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	_ "github.com/liukeshao/echo-template/ent/runtime"
	"github.com/liukeshao/echo-template/pkg/handlers"
	"github.com/liukeshao/echo-template/pkg/i18n"
	"github.com/liukeshao/echo-template/pkg/infra"
	"github.com/liukeshao/echo-template/pkg/patch"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"

	_ "modernc.org/sqlite"
)

func setupTestEcho(t *testing.T) (*echo.Echo, do.Injector, *services.AuthService) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))
	require.NoError(t, client.Schema.Create(context.Background()))
	t.Cleanup(func() { _ = client.Close() })

	cfg := &config.Config{
		App: config.AppConfig{
			Name:        "echo-template-test",
			Environment: config.EnvTest,
		},
		JWT: config.JWTConfig{
			Secret:             "test-secret-key",
			AccessTokenExpiry:  15 * time.Minute,
			RefreshTokenExpiry: 24 * time.Hour,
			Issuer:             "echo-template-test",
		},
		I18n: config.I18nConfig{
			DefaultLocale:    "en",
			SupportedLocales: []string{"en", "zh-CN"},
		},
	}

	inj := do.New(i18n.Package, services.Package, handlers.Package)
	do.ProvideValue(inj, cfg)
	do.ProvideValue(inj, &infra.ORM{Client: client})

	web := echo.New()
	require.NoError(t, handlers.BuildRouter(inj, web))

	return web, inj, do.MustInvoke[*services.AuthService](inj)
}

func TestServer_ShutdownBeforeStartDoesNotDeadlock(t *testing.T) {
	_, inj, _ := setupTestEcho(t)
	srv := do.MustInvoke[*handlers.Server](inj)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx))
}

func TestRouter_404RunsGlobalMiddleware(t *testing.T) {
	web, _, _ := setupTestEcho(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil)
	req.Header.Set("Accept-Language", "zh-CN")
	rec := httptest.NewRecorder()
	web.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID), "404 should still run RequestID middleware")

	var resp handlers.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, http.StatusNotFound, resp.Error.Code)
	assert.Equal(t, "NOT_FOUND", resp.Error.Status)
}

func TestBookHandler_PatchTypeMismatchReturns400(t *testing.T) {
	web, _, authSvc := setupTestEcho(t)

	authResp, err := authSvc.Register(context.Background(), &types.RegisterRequest{
		Username: "patcher",
		Email:    "patcher@example.com",
		Password: "password123",
	})
	require.NoError(t, err)

	// Create a book first.
	createBody := bytes.NewBufferString(`{"title":"Original","author":"Author","price":10}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/books", createBody)
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createReq.Header.Set("Authorization", "Bearer "+authResp.AccessToken)
	createRec := httptest.NewRecorder()
	web.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created types.BookResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	// Send PATCH with wrong type for price ("not-a-number") -> must return 400 PATCH_INVALID, not 500.
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/books/"+created.ID, bytes.NewBufferString(`{"price":"not-a-number"}`))
	patchReq.Header.Set(echo.HeaderContentType, patch.ContentTypeMergePatch)
	patchReq.Header.Set("Authorization", "Bearer "+authResp.AccessToken)
	patchRec := httptest.NewRecorder()
	web.ServeHTTP(patchRec, patchReq)

	assert.Equal(t, http.StatusBadRequest, patchRec.Code)
	var errResp handlers.ErrorResponse
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_ARGUMENT", errResp.Error.Status)
}

func TestDocsHandler_StaticIsolatesAssets(t *testing.T) {
	web, _, _ := setupTestEcho(t)

	// /docs, /openapi.yaml, and embedded JS/CSS assets are served.
	for _, p := range []string{"/docs", "/openapi.yaml", "/static/js/web-components.min.js", "/static/css/styles.min.css"} {
		rec := httptest.NewRecorder()
		web.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		assert.Equal(t, http.StatusOK, rec.Code, "path=%s", p)
	}

	// /static/* must not expose index.html, openapi.yaml, or directory listings.
	for _, p := range []string{"/static/", "/static/js/", "/static/index.html", "/static/openapi/openapi.yaml"} {
		rec := httptest.NewRecorder()
		web.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		assert.Equal(t, http.StatusNotFound, rec.Code, "path=%s", p)
	}
}

func TestRouter_BodyLimitMapsToResourceExhausted(t *testing.T) {
	web, _, _ := setupTestEcho(t)

	oversized := bytes.Repeat([]byte("a"), (2<<20)+1024)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(oversized))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	web.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	var errResp handlers.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "RESOURCE_EXHAUSTED", errResp.Error.Status)
}

func TestRouter_AuthBoundary(t *testing.T) {
	web, _, _ := setupTestEcho(t)

	// Public auth routes must NOT be blocked by RequireAuth (bad payload -> 400 validation, not 401).
	for _, p := range []string{"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/refresh"} {
		req := httptest.NewRequest(http.MethodPost, p, bytes.NewBufferString(`{}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		web.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "public route %s should reach validation without auth", p)
	}

	// Unmatched path under /api/v1/auth must return 404 (not 401).
	rec := httptest.NewRecorder()
	web.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/does-not-exist", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Protected routes require auth.
	protectedCases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodGet, "/api/v1/me"},
		{http.MethodPut, "/api/v1/me"},
		{http.MethodPatch, "/api/v1/me"},
		{http.MethodPost, "/api/v1/me/change-password"},
		{http.MethodPost, "/api/v1/books"},
		{http.MethodGet, "/api/v1/books"},
		{http.MethodGet, "/api/v1/books/01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{http.MethodPut, "/api/v1/books/01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{http.MethodPatch, "/api/v1/books/01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{http.MethodDelete, "/api/v1/books/01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	}
	for _, tc := range protectedCases {
		rec := httptest.NewRecorder()
		web.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s must require auth", tc.method, tc.path)
	}
}
