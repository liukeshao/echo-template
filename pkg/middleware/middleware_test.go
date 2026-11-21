package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/i18n"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/testutil"
)

func TestRequireAuth(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*services.AuthService](env.Injector)
	tok := env.MustAuthUser("mwuser", "mw@example.com", "password123")

	mw := middleware.RequireAuth(authSvc)

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
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := mw(func(_ *echo.Context) error { return nil })(c)
			testutil.AssertOopsCode(t, err, tc.wantCode)
		})
	}

	// Valid bearer token attaches authenticated user to context.
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var gotUsername string
	err := mw(func(c *echo.Context) error {
		gotUsername = appctx.MustGetUserFromContext(c.Request().Context()).Username
		return nil
	})(c)
	require.NoError(t, err)
	assert.Equal(t, "mwuser", gotUsername)
}

func TestLocaleMiddleware(t *testing.T) {
	t.Parallel()
	env := testutil.New(t)
	tr := do.MustInvoke[i18n.Translator](env.Injector)
	mw := middleware.Locale(tr)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/?lang=zh-CN", nil)
	req.Header.Set("Accept-Language", "en-US")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var got language.Tag
	err := mw(func(c *echo.Context) error {
		got = appctx.LocaleFromContext(c.Request().Context(), language.English)
		return nil
	})(c)
	require.NoError(t, err)
	assert.Equal(t, "zh-CN", got.String())
}

func TestRequestContextMiddleware(t *testing.T) {
	t.Parallel()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	rec.Header().Set(echo.HeaderXRequestID, "rid-999")
	c := e.NewContext(req, rec)

	var got string
	err := middleware.RequestContext()(func(c *echo.Context) error {
		got = appctx.MustGetRequestIDFromContext(c.Request().Context())
		return nil
	})(c)
	require.NoError(t, err)
	assert.Equal(t, "rid-999", got)
}
