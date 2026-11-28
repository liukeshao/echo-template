package service_test

import (
	"context"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/go-rest-template/internal/auth"
	"github.com/liukeshao/go-rest-template/internal/service"
	"github.com/liukeshao/go-rest-template/internal/testkit"
	"github.com/liukeshao/go-rest-template/pkg/httperr"
)

func TestUser_GetAndReplace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testkit.New(t)
	userSvc := do.MustInvoke[*service.User](env.Injector)
	jwtMgr := do.MustInvoke[*auth.JWT](env.Injector)

	reg := env.MustAuthUser("bob", "bob@example.com", "password123")
	other := env.MustAuthUser("carol", "carol@example.com", "password123")

	p, err := jwtMgr.ParseAccessToken(reg.AccessToken)
	require.NoError(t, err)
	otherP, err := jwtMgr.ParseAccessToken(other.AccessToken)
	require.NoError(t, err)
	assert.NotEqual(t, p.UserID, otherP.UserID)

	// Get returns persisted user profile.
	u, err := userSvc.Get(ctx, p.UserID)
	require.NoError(t, err)
	assert.Equal(t, "bob", u.Username)
	assert.Equal(t, "bob@example.com", u.Email)

	// Unknown user ID returns 404 NotFound with ResourceInfo(user).
	_, err = userSvc.Get(ctx, "01900000-0000-7000-8000-000000000000")
	oe := testkit.AssertOopsStatus(t, err, httperr.NotFound)
	assert.Equal(t, httperr.ResourceInfo{
		ResourceType: "user",
		ResourceName: "01900000-0000-7000-8000-000000000000",
	}, oe.Context()[httperr.MetaKey])

	// Replace normalizes email and updates username.
	updated, err := userSvc.Replace(ctx, p.UserID, service.ReplaceUserInput{
		Username: "bob_new",
		Email:    "Bob_New@Example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "bob_new", updated.Username)
	assert.Equal(t, "bob_new@example.com", updated.Email)

	// Conflicting email (case-insensitive) or username returns 409 Conflict.
	_, err = userSvc.Replace(ctx, p.UserID, service.ReplaceUserInput{
		Username: "bob_new",
		Email:    "CAROL@example.com",
	})
	oe = testkit.AssertOopsStatus(t, err, httperr.Conflict)
	assert.Equal(t, httperr.ResourceInfo{ResourceType: "user", ResourceName: "carol@example.com", Field: "email"}, oe.Context()[httperr.MetaKey])

	_, err = userSvc.Replace(ctx, p.UserID, service.ReplaceUserInput{
		Username: "carol",
		Email:    "bob_new@example.com",
	})
	oe = testkit.AssertOopsStatus(t, err, httperr.Conflict)
	assert.Equal(t, httperr.ResourceInfo{ResourceType: "user", ResourceName: "carol", Field: "username"}, oe.Context()[httperr.MetaKey])
}
