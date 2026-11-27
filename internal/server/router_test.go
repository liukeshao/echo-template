package server_test

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

	"github.com/liukeshao/echo-template/internal/handler"
	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/internal/server"
	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/httpkit"
)

func TestServer_ShutdownBeforeStartDoesNotDeadlock(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	srv := do.MustInvoke[*httpkit.Server](env.Injector)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx))
}

func TestBearerAuth(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*service.Auth](env.Injector)
	tr := do.MustInvoke[i18n.Translator](env.Injector)
	tok := env.MustAuthUser("mwuser", "mw@example.com", "password123")

	cfg := httpkit.ApplyHandlerOptions(
		httpkit.WithErrorEncoder(server.NewProblemEncoder(tr)),
		httpkit.WithAuthScheme("BearerAuth", server.BearerAuth(authSvc)),
	)
	op := httpkit.Operation{
		ID:      "testProtected",
		Method:  http.MethodGet,
		Pattern: "/",
		Auth:    [][]string{{"BearerAuth"}},
	}

	cases := []struct {
		name     string
		header   string
		wantType string
	}{
		{"missing header", "", httperr.PreconditionMissingHeader},
		{"wrong scheme", "Basic abc", httperr.PreconditionMalformedHeader},
		{"empty bearer", "Bearer ", httperr.PreconditionEmptyToken},
		{"invalid jwt", "Bearer not.a.jwt", httperr.PreconditionTokenInvalid},
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
			p := testutil.AssertProblem(t, rec, http.StatusUnauthorized)
			meta := testutil.DecodeProblemMeta[httperr.PreconditionInfo](t, p)
			assert.Equal(t, tc.wantType, meta.Type)
			assert.Equal(t, "authorization", meta.Subject)
		})
	}

	var gotUsername string
	h := cfg.Wrap(op, func(w http.ResponseWriter, r *http.Request) {
		gotUsername = handler.MustUser(r.Context()).Username
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
	assert.NotEmpty(t, rec.Header().Get(httpkit.HeaderRequestID), "404 should still run RequestID middleware")
	p := testutil.AssertProblem(t, rec, http.StatusNotFound)
	assert.Equal(t, "/api/v1/does-not-exist", p.Instance)
	nfMeta := testutil.DecodeProblemMeta[httperr.ResourceInfo](t, p)
	assert.Equal(t, httperr.ResourceInfo{ResourceType: "route", ResourceName: "/api/v1/does-not-exist"}, nfMeta)

	rec = env.DoRequest(http.MethodPut, "/api/v1/books")
	p = testutil.AssertProblem(t, rec, http.StatusMethodNotAllowed)
	methodMeta := testutil.DecodeProblemMeta[httperr.MethodInfo](t, p)
	assert.Equal(t, http.MethodPut, methodMeta.Method)
	assert.Equal(t, "/api/v1/books", methodMeta.Path)
	assert.NotEmpty(t, methodMeta.Allowed)
}

func TestRouter_BodyLimitReturns413(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)

	oversized := bytes.Repeat([]byte("a"), (2<<20)+1024)
	rec := env.DoRequest(http.MethodPost, "/api/v1/auth/login",
		testutil.WithRawBody("application/json", bytes.NewReader(oversized)),
	)
	p := testutil.AssertProblem(t, rec, http.StatusRequestEntityTooLarge)
	limitMeta := testutil.DecodeProblemMeta[httperr.RequestLimitInfo](t, p)
	assert.Equal(t, int64(2<<20), limitMeta.MaxBytes)
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
