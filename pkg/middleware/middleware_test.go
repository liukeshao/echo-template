package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/httpruntime"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/testutil"
)

func TestRequireAuth(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*services.AuthService](env.Injector)
	rt := do.MustInvoke[*httpruntime.Runtime](env.Injector)
	tok := env.MustAuthUser("mwuser", "mw@example.com", "password123")

	mw := middleware.RequireAuth(authSvc, rt)

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
			h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

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

	// Valid bearer token attaches authenticated user to context.
	var gotUsername string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUsername = appctx.MustGetUserFromContext(r.Context()).Username
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "mwuser", gotUsername)
}
