package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/handlers"
	"github.com/liukeshao/echo-template/pkg/httpruntime"
	"github.com/liukeshao/echo-template/pkg/patch"
	"github.com/liukeshao/echo-template/pkg/testutil"
)

func TestServer_ShutdownBeforeStartDoesNotDeadlock(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	srv := do.MustInvoke[*handlers.Server](env.Injector)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx))
}

func TestRouter_404And405RunGlobalMiddleware(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)

	rec := env.DoRequest(http.MethodGet, "/api/v1/does-not-exist",
		testutil.WithHeader("Accept-Language", "zh-CN"),
	)
	assert.NotEmpty(t, rec.Header().Get(httpruntime.HeaderRequestID), "404 should still run RequestID middleware")
	p := testutil.AssertProblem(t, rec, http.StatusNotFound, "NOT_FOUND")
	assert.Equal(t, "/api/v1/does-not-exist", p.Instance)

	rec = env.DoRequest(http.MethodPut, "/api/v1/books")
	testutil.AssertProblem(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
}

func TestBookHandler_PatchErrorsReturnProperHTTPStatus(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	authResp := env.MustAuthUser("patcher", "patcher@example.com", "password123")

	createRec := env.DoRequest(http.MethodPost, "/api/v1/books",
		testutil.WithJSON(map[string]any{"title": "Original", "author": "Author", "price": 10}),
		testutil.WithBearer(authResp.AccessToken),
	)
	require.Equal(t, http.StatusCreated, createRec.Code)
	created := testutil.DecodeJSON[api.Book](t, createRec)

	// Wrong type for price -> 400 PATCH_INVALID (not 500).
	patchRec := env.DoRequest(http.MethodPatch, "/api/v1/books/"+created.ID,
		testutil.WithBody(patch.ContentTypeMergePatch, `{"price":"not-a-number"}`),
		testutil.WithBearer(authResp.AccessToken),
	)
	testutil.AssertProblem(t, patchRec, http.StatusBadRequest, "PATCH_INVALID")

	// Unsupported PATCH Content-Type -> 415 PATCH_UNSUPPORTED_MEDIA_TYPE.
	unsupportedRec := env.DoRequest(http.MethodPatch, "/api/v1/books/"+created.ID,
		testutil.WithBody("text/plain", `title=foo`),
		testutil.WithBearer(authResp.AccessToken),
	)
	testutil.AssertProblem(t, unsupportedRec, http.StatusUnsupportedMediaType, "PATCH_UNSUPPORTED_MEDIA_TYPE")
}

func TestDocsHandler_StaticIsolatesAssets(t *testing.T) {
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

func TestRouter_BodyLimitReturns413(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)

	oversized := bytes.Repeat([]byte("a"), (2<<20)+1024)
	rec := env.DoRequest(http.MethodPost, "/api/v1/auth/login",
		testutil.WithRawBody("application/json", bytes.NewReader(oversized)),
	)
	testutil.AssertProblem(t, rec, http.StatusRequestEntityTooLarge, "REQUEST_ENTITY_TOO_LARGE")
}

func TestRouter_AuthBoundary(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)

	// Public auth routes must NOT be blocked by RequireAuth (bad payload -> 400 validation, not 401).
	for _, p := range []string{"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/refresh"} {
		rec := env.DoRequest(http.MethodPost, p, testutil.WithBody("application/json", `{}`))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "public route %s should reach validation without auth", p)
	}

	// Unmatched path under /api/v1/auth must return 404 (not 401).
	rec := env.DoRequest(http.MethodGet, "/api/v1/auth/does-not-exist")
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
		rec := env.DoRequest(tc.method, tc.path)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s must require auth", tc.method, tc.path)
	}
}

func TestDocsHandler_DisabledInProduction(t *testing.T) {
	t.Parallel()
	env := testutil.New(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.App.Environment = config.EnvProduction
	}))

	for _, p := range []string{"/docs", "/openapi.yaml", "/static/js/web-components.min.js"} {
		rec := env.DoRequest(http.MethodGet, p)
		assert.Equal(t, http.StatusNotFound, rec.Code, "path=%s must be disabled in prod", p)
	}
}

func TestAuthHandler_HTTPMethods(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)

	// Register
	rec := env.DoRequest(http.MethodPost, "/api/v1/auth/register",
		testutil.WithJSON(map[string]any{"username": "httpuser", "email": "http@example.com", "password": "password123"}),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	reg := testutil.DecodeJSON[api.TokenPair](t, rec)
	assert.NotEmpty(t, reg.AccessToken)

	// Login
	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/login",
		testutil.WithJSON(map[string]any{"email": "http@example.com", "password": "password123"}),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	login := testutil.DecodeJSON[api.TokenPair](t, rec)

	// Refresh
	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/refresh",
		testutil.WithJSON(map[string]any{"refresh_token": login.RefreshToken}),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	refreshed := testutil.DecodeJSON[api.TokenPair](t, rec)

	// Logout -> 204
	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/logout", testutil.WithBearer(refreshed.AccessToken))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestBookHandler_CRUDAndAllPatchProtocols(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	tok := env.MustAuthUser("bookuser", "bookuser@example.com", "password123")
	auth := testutil.WithBearer(tok.AccessToken)

	// Create
	rec := env.DoRequest(http.MethodPost, "/api/v1/books",
		testutil.WithJSON(map[string]any{"title": "Go", "author": "Alan", "description": "Desc", "price": 20}),
		auth,
	)
	require.Equal(t, http.StatusCreated, rec.Code)
	b := testutil.DecodeJSON[api.Book](t, rec)

	// Get
	rec = env.DoRequest(http.MethodGet, "/api/v1/books/"+b.ID, auth)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Go", testutil.DecodeJSON[api.Book](t, rec).Title)

	// Get invalid ULID -> 400 VALIDATION_INVALID
	rec = env.DoRequest(http.MethodGet, "/api/v1/books/not-a-ulid", auth)
	testutil.AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_INVALID")

	// List
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?page=1&page_size=10&order_by=title&filter=price%20%3E%2010", auth)
	require.Equal(t, http.StatusOK, rec.Code)
	list := testutil.DecodeJSON[api.BookList](t, rec)
	assert.Equal(t, int32(1), list.TotalSize)
	require.Len(t, list.Items, 1)
	assert.Equal(t, b.ID, list.Items[0].ID)

	// Replace (PUT)
	rec = env.DoRequest(http.MethodPut, "/api/v1/books/"+b.ID,
		testutil.WithJSON(map[string]any{"title": "Go 2", "author": "Alan", "price": 30}),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got := testutil.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, "Go 2", got.Title)
	assert.Empty(t, got.Description)

	// PATCH RFC 7386 Merge Patch + attempt to mutate read-only id (must be ignored)
	rec = env.DoRequest(http.MethodPatch, "/api/v1/books/"+b.ID,
		testutil.WithBody(patch.ContentTypeMergePatch, `{"id":"FAKE","title":"Merged","price":null}`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got = testutil.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, b.ID, got.ID)
	assert.Equal(t, "Merged", got.Title)
	assert.Nil(t, got.Price)

	// PATCH RFC 6902 JSON Patch
	rec = env.DoRequest(http.MethodPatch, "/api/v1/books/"+b.ID,
		testutil.WithBody(patch.ContentTypeJSONPatch, `[{"op":"replace","path":"/title","value":"JSONPatched"}]`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "JSONPatched", testutil.DecodeJSON[api.Book](t, rec).Title)

	// PATCH Shorthand
	rec = env.DoRequest(http.MethodPatch, "/api/v1/books/"+b.ID,
		testutil.WithBody(patch.ContentTypeShorthandPatch, `title: "Shorthand", price: 15.5`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got = testutil.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, "Shorthand", got.Title)
	require.NotNil(t, got.Price)
	assert.InDelta(t, 15.5, *got.Price, 1e-9)

	// Delete -> 204
	rec = env.DoRequest(http.MethodDelete, "/api/v1/books/"+b.ID, auth)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestMeHandler_HTTPMethods(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	tok := env.MustAuthUser("meuser", "meuser@example.com", "password123")
	auth := testutil.WithBearer(tok.AccessToken)

	// GET /me
	rec := env.DoRequest(http.MethodGet, "/api/v1/me", auth)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "meuser", testutil.DecodeJSON[api.User](t, rec).Username)

	// PUT /me
	rec = env.DoRequest(http.MethodPut, "/api/v1/me",
		testutil.WithJSON(map[string]any{"username": "meuser2", "email": "meuser2@example.com"}),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "meuser2", testutil.DecodeJSON[api.User](t, rec).Username)

	// PATCH /me (auto-generated by @autoPatch(Me.get, Me.replace))
	rec = env.DoRequest(http.MethodPatch, "/api/v1/me",
		testutil.WithBody(patch.ContentTypeMergePatch, `{"email":"meuser3@example.com"}`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "meuser3@example.com", testutil.DecodeJSON[api.User](t, rec).Email)

	// POST /me/change-password -> 204
	rec = env.DoRequest(http.MethodPost, "/api/v1/me/change-password",
		testutil.WithJSON(map[string]any{"old_password": "password123", "new_password": "newpassword123"}),
		auth,
	)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
