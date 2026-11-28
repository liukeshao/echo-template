package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/go-rest-template/config"
	"github.com/liukeshao/go-rest-template/ent/refreshtoken"
	entuser "github.com/liukeshao/go-rest-template/ent/user"
	"github.com/liukeshao/go-rest-template/internal/auth"
	"github.com/liukeshao/go-rest-template/internal/service"
	"github.com/liukeshao/go-rest-template/internal/testkit"
	"github.com/liukeshao/go-rest-template/pkg/api"
	"github.com/liukeshao/go-rest-template/pkg/httperr"
)

func TestAuth_FullLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testkit.New(t)
	authSvc := do.MustInvoke[*service.Auth](env.Injector)
	jwtMgr := do.MustInvoke[*auth.JWT](env.Injector)

	// Register normalizes email case, creates user, returns stateless Access JWT, and stores 1 hashed RefreshToken.
	regResp, err := authSvc.Register(ctx, &api.RegisterRequest{
		Username: "alice",
		Email:    "Alice@Example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, regResp.AccessToken)
	assert.NotEmpty(t, regResp.RefreshToken)

	principal, err := jwtMgr.ParseAccessToken(regResp.AccessToken)
	require.NoError(t, err)
	assert.NotEmpty(t, principal.UserID)

	storedTokens, err := env.ORM().RefreshToken.Query().
		Where(refreshtoken.UserID(principal.UserID)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, storedTokens, 1, "only refresh token must be persisted (not access token)")
	assert.Len(t, storedTokens[0].TokenHash, 64, "refresh token must be stored as SHA-256 hex digest")
	assert.NotEqual(t, regResp.RefreshToken, storedTokens[0].TokenHash, "plaintext refresh token must never be stored")

	// Duplicate email (case-insensitive) / username rejected with httperr.Conflict + ResourceInfo.
	_, err = authSvc.Register(ctx, &api.RegisterRequest{
		Username: "alice2",
		Email:    "alice@example.com",
		Password: "password123",
	})
	oe := testkit.AssertOopsStatus(t, err, httperr.Conflict)
	assert.Equal(t, httperr.ResourceInfo{ResourceType: "user", ResourceName: "alice@example.com", Field: "email"}, oe.Context()[httperr.MetaKey])

	_, err = authSvc.Register(ctx, &api.RegisterRequest{
		Username: "alice",
		Email:    "alice2@example.com",
		Password: "password123",
	})
	oe = testkit.AssertOopsStatus(t, err, httperr.Conflict)
	assert.Equal(t, httperr.ResourceInfo{ResourceType: "user", ResourceName: "alice", Field: "username"}, oe.Context()[httperr.MetaKey])

	// Two logins within the same second (including mixed email case) must succeed and not collide.
	login1, err := authSvc.Login(ctx, &api.LoginRequest{
		Email:    "ALICE@EXAMPLE.COM",
		Password: "password123",
	})
	require.NoError(t, err)
	login2, err := authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.NotEqual(t, login1.AccessToken, login2.AccessToken)
	assert.NotEqual(t, login1.RefreshToken, login2.RefreshToken)

	// RefreshToken rotates atomically within the same family.
	refreshed, err := authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: login1.RefreshToken,
	})
	require.NoError(t, err)
	assert.NotEqual(t, login1.RefreshToken, refreshed.RefreshToken)

	// Replaying the already-rotated refresh token triggers reuse detection:
	// returns token_revoked and revokes the entire login1 family (including refreshed.RefreshToken),
	// while keeping the independent login2 family intact.
	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: login1.RefreshToken,
	})
	oe = testkit.AssertOopsStatus(t, err, httperr.Unauthorized)
	assert.Equal(t, httperr.PreconditionInfo{Type: auth.ReasonTokenRevoked, Subject: "refresh_token"}, oe.Context()[httperr.MetaKey])

	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: refreshed.RefreshToken,
	})
	oe = testkit.AssertOopsStatus(t, err, httperr.Unauthorized)
	assert.Equal(t, httperr.PreconditionInfo{Type: auth.ReasonTokenRevoked, Subject: "refresh_token"}, oe.Context()[httperr.MetaKey])

	login2Refreshed, err := authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: login2.RefreshToken,
	})
	require.NoError(t, err)

	// Logout revokes all remaining active refresh tokens for the user.
	require.NoError(t, authSvc.Logout(ctx, principal.UserID))
	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: login2Refreshed.RefreshToken,
	})
	testkit.AssertOopsStatus(t, err, httperr.Unauthorized)

	// ChangePassword rejects wrong old password (400) and revokes active refresh tokens on success.
	login3, err := authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	require.NoError(t, err)

	err = authSvc.ChangePassword(ctx, principal.UserID, &api.ChangePasswordRequest{
		OldPassword: "wrongpassword1",
		NewPassword: "newpassword123",
	})
	oe = testkit.AssertOopsStatus(t, err, httperr.BadRequest)
	assert.Equal(t, httperr.PreconditionInfo{Type: service.ReasonInvalidPassword, Subject: "old_password"}, oe.Context()[httperr.MetaKey])

	require.NoError(t, authSvc.ChangePassword(ctx, principal.UserID, &api.ChangePasswordRequest{
		OldPassword: "password123",
		NewPassword: "newpassword123",
	}))
	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: login3.RefreshToken,
	})
	testkit.AssertOopsStatus(t, err, httperr.Unauthorized)

	// Disabled account: wrong password must still return invalid credentials (no account enumeration),
	// while right password returns httperr.Forbidden + account_disabled.
	require.NoError(t, env.ORM().User.UpdateOneID(principal.UserID).SetStatus(entuser.StatusSuspended).Exec(ctx))
	_, err = authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "wrongpassword1",
	})
	oe = testkit.AssertOopsStatus(t, err, httperr.Unauthorized)
	assert.Equal(t, httperr.PreconditionInfo{Type: service.ReasonInvalidCredentials}, oe.Context()[httperr.MetaKey])

	_, err = authSvc.Login(ctx, &api.LoginRequest{
		Email:    "alice@example.com",
		Password: "newpassword123",
	})
	oe = testkit.AssertOopsStatus(t, err, httperr.Forbidden)
	assert.Equal(t, httperr.PreconditionInfo{Type: service.ReasonAccountDisabled, Subject: "account", Detail: string(entuser.StatusSuspended)}, oe.Context()[httperr.MetaKey])
}

func TestAuth_TokenTypeMismatchAndErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testkit.New(t)
	authSvc := do.MustInvoke[*service.Auth](env.Injector)
	jwtMgr := do.MustInvoke[*auth.JWT](env.Injector)
	tok := env.MustAuthUser("tokuser", "tokuser@example.com", "password123")

	// Passing opaque refresh token as Bearer Access JWT must be rejected.
	_, err := jwtMgr.ParseAccessToken(tok.RefreshToken)
	testkit.AssertOopsStatus(t, err, httperr.Unauthorized)

	// Passing Access JWT to RefreshToken must be rejected.
	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{RefreshToken: tok.AccessToken})
	testkit.AssertOopsStatus(t, err, httperr.Unauthorized)

	// Unknown email login returns httperr.Unauthorized + invalid_credentials.
	_, err = authSvc.Login(ctx, &api.LoginRequest{Email: "nobody@example.com", Password: "password123"})
	oe := testkit.AssertOopsStatus(t, err, httperr.Unauthorized)
	assert.Equal(t, httperr.PreconditionInfo{Type: service.ReasonInvalidCredentials}, oe.Context()[httperr.MetaKey])
}

func TestAuth_ExpiredTokensReportTokenExpiredMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testkit.New(t, testkit.WithConfig(func(cfg *config.Config) {
		cfg.Auth.JWT.AccessTokenExpiry = -time.Second
		cfg.Auth.JWT.RefreshTokenExpiry = -time.Second
	}))
	authSvc := do.MustInvoke[*service.Auth](env.Injector)
	jwtMgr := do.MustInvoke[*auth.JWT](env.Injector)
	tok := env.MustAuthUser("expuser", "exp@example.com", "password123")

	_, err := jwtMgr.ParseAccessToken(tok.AccessToken)
	oe := testkit.AssertOopsStatus(t, err, httperr.Unauthorized)
	assert.ErrorIs(t, err, jwt.ErrTokenExpired, "underlying JWT expiration cause must stay in errors.Is chain")
	assert.Equal(t, httperr.PreconditionInfo{Type: auth.ReasonTokenExpired, Subject: "authorization", Challenge: "Bearer"}, oe.Context()[httperr.MetaKey])

	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{RefreshToken: tok.RefreshToken})
	oe = testkit.AssertOopsStatus(t, err, httperr.Unauthorized)
	assert.Equal(t, httperr.PreconditionInfo{Type: auth.ReasonTokenExpired, Subject: "refresh_token"}, oe.Context()[httperr.MetaKey])
}
