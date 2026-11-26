package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/rest"
)

func TestAuth_FullLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*service.Auth](env.Injector)

	// Register creates user and persists both tokens with UUID v7 IDs.
	regResp, err := authSvc.Register(ctx, &api.RegisterRequest{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, regResp.AccessToken)
	assert.NotEmpty(t, regResp.RefreshToken)

	// Duplicate email / username rejected with CodeConflict + ResourceInfo.
	_, err = authSvc.Register(ctx, &api.RegisterRequest{
		Username: "alice2",
		Email:    "alice@example.com",
		Password: "password123",
	})
	oe := testutil.AssertOopsCode(t, err, rest.CodeConflict)
	assert.Equal(t, rest.ResourceInfo{ResourceType: "user", ResourceName: "alice@example.com", Field: "email"}, oe.Context()[rest.MetaKey])

	_, err = authSvc.Register(ctx, &api.RegisterRequest{
		Username: "alice",
		Email:    "alice2@example.com",
		Password: "password123",
	})
	oe = testutil.AssertOopsCode(t, err, rest.CodeConflict)
	assert.Equal(t, rest.ResourceInfo{ResourceType: "user", ResourceName: "alice", Field: "username"}, oe.Context()[rest.MetaKey])

	// Two logins within the same second must not collide (jti uniqueness).
	login1, err := authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	login2, err := authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.NotEqual(t, login1.AccessToken, login2.AccessToken)

	// AuthenticateUser works via single JOIN query.
	u, err := authSvc.AuthenticateUser(ctx, login1.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "alice", u.Username)

	// RefreshToken rotates atomically and rejects replay of the old refresh token.
	refreshed, err := authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: login1.RefreshToken,
	})
	require.NoError(t, err)
	assert.NotEqual(t, login1.RefreshToken, refreshed.RefreshToken)

	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: login1.RefreshToken,
	})
	oe = testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)
	assert.Equal(t, rest.PreconditionInfo{Type: rest.PreconditionTokenRevoked, Subject: "refresh_token"}, oe.Context()[rest.MetaKey])

	// Logout revokes all active access and refresh tokens for the user.
	require.NoError(t, authSvc.Logout(ctx, u.ID))
	_, err = authSvc.AuthenticateUser(ctx, refreshed.AccessToken)
	oe = testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)
	assert.Equal(t, rest.PreconditionInfo{Type: rest.PreconditionTokenRevoked, Subject: "authorization", Detail: "Bearer"}, oe.Context()[rest.MetaKey])
	_, err = authSvc.AuthenticateUser(ctx, login2.AccessToken)
	testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)
	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: refreshed.RefreshToken,
	})
	testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)

	// Disabled account: wrong password must still return invalid credentials (no account enumeration),
	// while right password returns CodeForbidden + ACCOUNT_DISABLED.
	require.NoError(t, env.ORM().User.UpdateOneID(u.ID).SetStatus(userEnt.StatusSuspended).Exec(ctx))
	_, err = authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "wrongpassword1",
	})
	oe = testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)
	assert.Equal(t, rest.PreconditionInfo{Type: rest.PreconditionInvalidCredentials}, oe.Context()[rest.MetaKey])

	_, err = authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	oe = testutil.AssertOopsCode(t, err, rest.CodeForbidden)
	assert.Equal(t, rest.PreconditionInfo{Type: rest.PreconditionAccountDisabled, Subject: "account", Detail: string(userEnt.StatusSuspended)}, oe.Context()[rest.MetaKey])
}

func TestAuth_TokenTypeMismatchAndErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*service.Auth](env.Injector)
	tok := env.MustAuthUser("tokuser", "tokuser@example.com", "password123")

	// Passing refresh token to AuthenticateUser must be rejected.
	_, err := authSvc.AuthenticateUser(ctx, tok.RefreshToken)
	testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)

	// Passing access token to RefreshToken must be rejected.
	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{RefreshToken: tok.AccessToken})
	testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)

	// Unknown email login returns CodeUnauthorized + INVALID_CREDENTIALS.
	_, err = authSvc.Login(ctx, &api.LoginRequest{Email: "nobody@example.com", Password: "password123"})
	oe := testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)
	assert.Equal(t, rest.PreconditionInfo{Type: rest.PreconditionInvalidCredentials}, oe.Context()[rest.MetaKey])
}

func TestAuth_ExpiredTokensReportTokenExpiredMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.JWT.AccessTokenExpiry = -time.Second
		cfg.JWT.RefreshTokenExpiry = -time.Second
	}))
	authSvc := do.MustInvoke[*service.Auth](env.Injector)
	tok := env.MustAuthUser("expuser", "exp@example.com", "password123")

	_, err := authSvc.AuthenticateUser(ctx, tok.AccessToken)
	oe := testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)
	assert.ErrorIs(t, err, jwt.ErrTokenExpired, "underlying JWT expiration cause must stay in errors.Is chain")
	assert.Equal(t, rest.PreconditionInfo{Type: rest.PreconditionTokenExpired, Subject: "authorization", Detail: "Bearer"}, oe.Context()[rest.MetaKey])

	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{RefreshToken: tok.RefreshToken})
	oe = testutil.AssertOopsCode(t, err, rest.CodeUnauthorized)
	assert.ErrorIs(t, err, jwt.ErrTokenExpired)
	assert.Equal(t, rest.PreconditionInfo{Type: rest.PreconditionTokenExpired, Subject: "refresh_token"}, oe.Context()[rest.MetaKey])
}
