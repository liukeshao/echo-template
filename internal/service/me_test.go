package service_test

import (
	"context"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/httperr"
)

func TestMe_ReplaceAndChangePassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*service.Auth](env.Injector)
	meSvc := do.MustInvoke[*service.Me](env.Injector)

	reg := env.MustAuthUser("bob", "bob@example.com", "password123")
	u, err := authSvc.AuthenticateUser(ctx, reg.AccessToken)
	require.NoError(t, err)

	updated, err := meSvc.Replace(ctx, u.ID, &api.ReplaceMeRequest{
		Username: "bob_new",
		Email:    "bob_new@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "bob_new", updated.Username)

	err = meSvc.ChangePassword(ctx, u.ID, &api.ChangePasswordRequest{
		OldPassword: "wrongpassword1",
		NewPassword: "newpassword123",
	})
	oe := testutil.AssertOopsStatus(t, err, httperr.BadRequest)
	assert.Equal(t, httperr.PreconditionInfo{Type: httperr.PreconditionInvalidPassword, Subject: "old_password"}, oe.Context()[httperr.MetaKey])

	require.NoError(t, meSvc.ChangePassword(ctx, u.ID, &api.ChangePasswordRequest{
		OldPassword: "password123",
		NewPassword: "newpassword123",
	}))

	// Changing password must revoke all active tokens.
	_, err = authSvc.AuthenticateUser(ctx, reg.AccessToken)
	testutil.AssertOopsStatus(t, err, httperr.Unauthorized)
	_, err = authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{
		RefreshToken: reg.RefreshToken,
	})
	testutil.AssertOopsStatus(t, err, httperr.Unauthorized)
}
