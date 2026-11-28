package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/go-rest-template/config"
	"github.com/liukeshao/go-rest-template/internal/auth"
	"github.com/liukeshao/go-rest-template/internal/server"
	"github.com/liukeshao/go-rest-template/internal/testkit"
	"github.com/liukeshao/go-rest-template/pkg/httperr"
	"github.com/liukeshao/go-rest-template/pkg/httpkit"
)

func TestPrincipalContext(t *testing.T) {
	t.Parallel()

	p := auth.Principal{UserID: "user-123", TokenID: "tok-456"}
	ctx := auth.WithPrincipal(context.Background(), p)

	got, ok := auth.PrincipalFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, p, got)
	assert.Equal(t, p, auth.MustPrincipal(ctx))

	// Missing or empty UserID returns false and panics in MustPrincipal.
	_, ok = auth.PrincipalFromContext(context.Background())
	assert.False(t, ok)

	_, ok = auth.PrincipalFromContext(auth.WithPrincipal(context.Background(), auth.Principal{}))
	assert.False(t, ok)

	assert.Panics(t, func() {
		auth.MustPrincipal(context.Background())
	})
}

func TestJWT_SignAndParseAccessToken(t *testing.T) {
	t.Parallel()

	cfg := config.JWT{
		Secret:            "test-secret-key-at-least-32-bytes-long",
		AccessTokenExpiry: 15 * time.Minute,
		Issuer:            "go-rest-template-test",
	}
	mgr := auth.NewJWT(cfg)

	tok, exp, err := mgr.SignAccessToken("user-1")
	require.NoError(t, err)
	assert.NotEmpty(t, tok)
	assert.True(t, exp.After(time.Now()))

	p, err := mgr.ParseAccessToken(tok)
	require.NoError(t, err)
	assert.Equal(t, "user-1", p.UserID)
	assert.NotEmpty(t, p.TokenID)

	// Two tokens signed in the same instant have distinct jti.
	tok2, _, err := mgr.SignAccessToken("user-1")
	require.NoError(t, err)
	assert.NotEqual(t, tok, tok2)

	// Wrong secret rejected as token_invalid.
	otherSecret := auth.NewJWT(config.JWT{
		Secret:            "different-secret-key-at-least-32-bytes",
		AccessTokenExpiry: 15 * time.Minute,
		Issuer:            cfg.Issuer,
	})
	_, err = otherSecret.ParseAccessToken(tok)
	oe := testkit.AssertOopsStatus(t, err, httperr.Unauthorized)
	assert.Equal(t, httperr.PreconditionInfo{
		Type:      auth.ReasonTokenInvalid,
		Subject:   "authorization",
		Challenge: "Bearer",
	}, oe.Context()[httperr.MetaKey])

	// Wrong issuer rejected.
	otherIssuer := auth.NewJWT(config.JWT{
		Secret:            cfg.Secret,
		AccessTokenExpiry: 15 * time.Minute,
		Issuer:            "other-issuer",
	})
	_, err = otherIssuer.ParseAccessToken(tok)
	testkit.AssertOopsStatus(t, err, httperr.Unauthorized)

	// Expired token preserves jwt.ErrTokenExpired and sets token_expired metadata.
	expMgr := auth.NewJWT(config.JWT{
		Secret:            cfg.Secret,
		AccessTokenExpiry: -time.Second,
		Issuer:            cfg.Issuer,
	})
	expiredTok, _, err := expMgr.SignAccessToken("user-1")
	require.NoError(t, err)
	_, err = mgr.ParseAccessToken(expiredTok)
	oe = testkit.AssertOopsStatus(t, err, httperr.Unauthorized)
	assert.ErrorIs(t, err, jwt.ErrTokenExpired)
	assert.Equal(t, httperr.PreconditionInfo{
		Type:      auth.ReasonTokenExpired,
		Subject:   "authorization",
		Challenge: "Bearer",
	}, oe.Context()[httperr.MetaKey])

	// Missing sub rejected.
	noSub, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    cfg.Issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}).SignedString([]byte(cfg.Secret))
	require.NoError(t, err)
	_, err = mgr.ParseAccessToken(noSub)
	testkit.AssertOopsStatus(t, err, httperr.Unauthorized)
}

func TestBearerAuthenticator(t *testing.T) {
	t.Parallel()

	mgr := auth.NewJWT(config.JWT{
		Secret:            "test-secret-key-at-least-32-bytes-long",
		AccessTokenExpiry: 15 * time.Minute,
		Issuer:            "go-rest-template-test",
	})
	tok, _, err := mgr.SignAccessToken("user-42")
	require.NoError(t, err)

	cfg := httpkit.ApplyHandlerOptions(
		httpkit.WithErrorEncoder(server.NewProblemEncoder(nil)),
		httpkit.WithAuthScheme("BearerAuth", auth.BearerAuthenticator(mgr)),
	)
	op := httpkit.Operation{
		ID:      "protected",
		Method:  http.MethodGet,
		Pattern: "/",
		Auth:    [][]string{{"BearerAuth"}},
	}

	var got auth.Principal
	h := cfg.Wrap(op, func(w http.ResponseWriter, r *http.Request) {
		got = auth.MustPrincipal(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Valid lowercase "bearer" scheme injects Principal with zero DB access.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-42", got.UserID)
	assert.NotEmpty(t, got.TokenID)

	// Invalid token returns 401 + WWW-Authenticate: Bearer.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
}
