package service_test

import (
	"context"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/api"
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

	// Duplicate email / username rejected.
	_, err = authSvc.Register(ctx, &api.RegisterRequest{
		Username: "alice2",
		Email:    "alice@example.com",
		Password: "password123",
	})
	testutil.AssertOopsCode(t, err, codes.AuthEmailTaken)

	_, err = authSvc.Register(ctx, &api.RegisterRequest{
		Username: "alice",
		Email:    "alice2@example.com",
		Password: "password123",
	})
	testutil.AssertOopsCode(t, err, codes.AuthUsernameTaken)

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
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)

	// Logout revokes all active access and refresh tokens for the user.
	require.NoError(t, authSvc.Logout(ctx, u.ID))
	_, err = authSvc.AuthenticateUser(ctx, refreshed.AccessToken)
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)
	_, err = authSvc.AuthenticateUser(ctx, login2.AccessToken)
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)
	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: refreshed.RefreshToken,
	})
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)

	// Disabled account: wrong password must still return invalid credentials (no account enumeration),
	// while right password returns account disabled.
	require.NoError(t, env.ORM().User.UpdateOneID(u.ID).SetStatus(userEnt.StatusSuspended).Exec(ctx))
	_, err = authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "wrongpassword1",
	})
	testutil.AssertOopsCode(t, err, codes.AuthInvalidCredentials)

	_, err = authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	testutil.AssertOopsCode(t, err, codes.AuthAccountDisabled)
}

func TestAuth_TokenTypeMismatchAndErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*service.Auth](env.Injector)
	tok := env.MustAuthUser("tokuser", "tokuser@example.com", "password123")

	// Passing refresh token to AuthenticateUser must be rejected.
	_, err := authSvc.AuthenticateUser(ctx, tok.RefreshToken)
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)

	// Passing access token to RefreshToken must be rejected.
	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{RefreshToken: tok.AccessToken})
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)

	// Unknown email login returns AuthInvalidCredentials.
	_, err = authSvc.Login(ctx, &api.LoginRequest{Email: "nobody@example.com", Password: "password123"})
	testutil.AssertOopsCode(t, err, codes.AuthInvalidCredentials)
}
