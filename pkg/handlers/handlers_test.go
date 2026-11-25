package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/handlers"
	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/testutil"
)

func TestServer_ShutdownBeforeStartDoesNotDeadlock(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	srv := do.MustInvoke[*rest.Server](env.Injector)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx))
}

func TestBearerAuth(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*services.AuthService](env.Injector)
	rt := do.MustInvoke[*rest.Runtime](env.Injector)
	tok := env.MustAuthUser("mwuser", "mw@example.com", "password123")

	cfg := rest.ApplyHandlerOptions(
		rest.WithRuntime(rt),
		rest.WithAuthScheme("BearerAuth", handlers.BearerAuth(authSvc)),
	)
	op := rest.Operation{
		ID:      "testProtected",
		Method:  http.MethodGet,
		Pattern: "/",
		Auth:    [][]string{{"BearerAuth"}},
	}

	cases := []struct {
		name     string
		header   string
		wantCode codes.Code
	}{
		{"missing header", "", codes.AuthTokenInvalid},
		{"wrong scheme", "Basic abc", codes.AuthTokenInvalid},
		{"empty bearer", "Bearer ", codes.AuthTokenInvalid},
		{"invalid jwt", "Bearer not.a.jwt", codes.AuthTokenInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := cfg.Wrap(op, func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			assert.False(t, called)
			testutil.AssertProblem(t, rec, http.StatusUnauthorized, tc.wantCode.String())
		})
	}

	var gotUsername string
	h := cfg.Wrap(op, func(w http.ResponseWriter, r *http.Request) {
		gotUsername = appctx.MustGetUserFromContext(r.Context()).Username
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "bearer "+tok.AccessToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "mwuser", gotUsername)
}

func TestRouter_404And405RunGlobalMiddleware(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)

	rec := env.DoRequest(http.MethodGet, "/api/v1/does-not-exist",
		testutil.WithHeader("Accept-Language", "zh-CN"),
	)
	assert.NotEmpty(t, rec.Header().Get(rest.HeaderRequestID), "404 should still run RequestID middleware")
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
		testutil.WithBody(rest.ContentTypeMergePatch, `{"price":"not-a-number"}`),
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
		{http.MethodGet, "/api/v1/books/0195a3b0-7c4e-7000-8000-000000000001"},
		{http.MethodPut, "/api/v1/books/0195a3b0-7c4e-7000-8000-000000000001"},
		{http.MethodPatch, "/api/v1/books/0195a3b0-7c4e-7000-8000-000000000001"},
		{http.MethodDelete, "/api/v1/books/0195a3b0-7c4e-7000-8000-000000000001"},
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

	// Register -> 201 Created
	rec := env.DoRequest(http.MethodPost, "/api/v1/auth/register",
		testutil.WithJSON(map[string]any{"username": "httpuser", "email": "http@example.com", "password": "password123"}),
	)
	require.Equal(t, http.StatusCreated, rec.Code)
	reg := testutil.DecodeJSON[api.TokenPair](t, rec)
	assert.NotEmpty(t, reg.AccessToken)

	// Malformed JSON -> localized 400 BAD_REQUEST; JSON type mismatch -> 400 VALIDATION_INVALID without leaking Go struct names
	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/login",
		testutil.WithBody("application/json", `{bad json`),
		testutil.WithHeader("Accept-Language", "zh-CN"),
	)
	p := testutil.AssertProblem(t, rec, http.StatusBadRequest, "BAD_REQUEST")
	assert.Equal(t, "请求体不是合法的 JSON 格式。", p.Detail)

	rec = env.DoRequest(http.MethodPost, "/api/v1/auth/login",
		testutil.WithBody("application/json", `{"email":123,"password":"x"}`),
	)
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_INVALID")
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "email", p.Errors[0].Field)
	assert.NotContains(t, p.Errors[0].Message, "LoginRequest")

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

func TestBookHandler_CRUDAndMergePatch(t *testing.T) {
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

	// Get invalid UUID -> 400 VALIDATION_INVALID
	rec = env.DoRequest(http.MethodGet, "/api/v1/books/not-a-uuid", auth)
	testutil.AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_INVALID")

	// List invalid integer query -> 400 VALIDATION_INVALID
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?page=abc", auth)
	p := testutil.AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_INVALID")
	require.Len(t, p.Errors, 1)
	assert.Contains(t, p.Errors[0].Message, "valid integer")

	// List invalid float query -> 400 VALIDATION_INVALID
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?min_price=not-a-number", auth)
	p = testutil.AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_INVALID")
	require.Len(t, p.Errors, 1)
	assert.Equal(t, "min_price", p.Errors[0].Field)
	assert.Contains(t, p.Errors[0].Message, "valid number")

	// List negative min_price -> 400 VALIDATION_INVALID
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?min_price=-1", auth)
	testutil.AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_INVALID")

	// List with explicit typed query fields
	rec = env.DoRequest(http.MethodGet, "/api/v1/books?page=1&page_size=10&order_by=title&title=go&min_price=10&max_price=30", auth)
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
		testutil.WithBody(rest.ContentTypeMergePatch, `{"id":"FAKE","title":"Merged","price":null}`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got = testutil.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, b.ID, got.ID)
	assert.Equal(t, "Merged", got.Title)
	assert.Nil(t, got.Price)

	// PATCH with application/json compatibility
	rec = env.DoRequest(http.MethodPatch, "/api/v1/books/"+b.ID,
		testutil.WithBody(rest.ContentTypeJSON, `{"title":"JSONCompat","price":15.5}`),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	got = testutil.DecodeJSON[api.Book](t, rec)
	assert.Equal(t, "JSONCompat", got.Title)
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
	me := testutil.DecodeJSON[api.User](t, rec)
	assert.Equal(t, "meuser", me.Username)
	assert.NotNil(t, me.LastLoginAt)

	// PUT /me
	rec = env.DoRequest(http.MethodPut, "/api/v1/me",
		testutil.WithJSON(map[string]any{"username": "meuser2", "email": "meuser2@example.com"}),
		auth,
	)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "meuser2", testutil.DecodeJSON[api.User](t, rec).Username)

	// PATCH /me (auto-generated by @autoPatch(Me.getMe, Me.replaceMe))
	rec = env.DoRequest(http.MethodPatch, "/api/v1/me",
		testutil.WithBody(rest.ContentTypeMergePatch, `{"email":"meuser3@example.com"}`),
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
