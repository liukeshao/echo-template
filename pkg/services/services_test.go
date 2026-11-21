package services_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/samber/do/v2"
	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	_ "github.com/liukeshao/echo-template/ent/runtime"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/infra"
	metav1 "github.com/liukeshao/echo-template/pkg/meta/v1"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
	"github.com/liukeshao/echo-template/pkg/validator"

	_ "modernc.org/sqlite"
)

func setupServices(t *testing.T) (*ent.Client, *services.AuthService, *services.BookService, *services.MeService) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))
	require.NoError(t, client.Schema.Create(context.Background()))
	t.Cleanup(func() { _ = client.Close() })

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:             "test-secret-key-for-unit-tests",
			AccessTokenExpiry:  15 * time.Minute,
			RefreshTokenExpiry: 24 * time.Hour,
			Issuer:             "echo-template-test",
		},
	}

	inj := do.New(services.Package)
	do.ProvideValue(inj, cfg)
	do.ProvideValue(inj, &infra.ORM{Client: client})

	authSvc := do.MustInvoke[*services.AuthService](inj)
	bookSvc := do.MustInvoke[*services.BookService](inj)
	meSvc := do.MustInvoke[*services.MeService](inj)

	return client, authSvc, bookSvc, meSvc
}

func assertOopsCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	require.Error(t, err)
	var oe oops.OopsError
	require.True(t, errors.As(err, &oe), "expected oops error, got %T: %v", err, err)
	assert.Equal(t, want.String(), oe.Code())
}

func TestAuthService_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	client, authSvc, _, _ := setupServices(t)

	// Register creates user and persists both tokens with ULID IDs.
	regResp, err := authSvc.Register(ctx, &types.RegisterRequest{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, regResp.AccessToken)
	assert.NotEmpty(t, regResp.RefreshToken)

	// Duplicate email / username rejected.
	_, err = authSvc.Register(ctx, &types.RegisterRequest{
		Username: "alice2",
		Email:    "alice@example.com",
		Password: "password123",
	})
	assertOopsCode(t, err, codes.AuthEmailTaken)

	_, err = authSvc.Register(ctx, &types.RegisterRequest{
		Username: "alice",
		Email:    "alice2@example.com",
		Password: "password123",
	})
	assertOopsCode(t, err, codes.AuthUsernameTaken)

	// Two logins within the same second must not collide (jti uniqueness).
	login1, err := authSvc.Login(ctx, &types.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	login2, err := authSvc.Login(ctx, &types.LoginRequest{
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
	refreshed, err := authSvc.RefreshToken(ctx, &types.RefreshTokenRequest{
		RefreshToken: login1.RefreshToken,
	})
	require.NoError(t, err)
	assert.NotEqual(t, login1.RefreshToken, refreshed.RefreshToken)

	_, err = authSvc.RefreshToken(ctx, &types.RefreshTokenRequest{
		RefreshToken: login1.RefreshToken,
	})
	assertOopsCode(t, err, codes.AuthTokenInvalid)

	// Logout revokes all active access and refresh tokens for the user.
	require.NoError(t, authSvc.Logout(ctx, u.ID))
	_, err = authSvc.AuthenticateUser(ctx, refreshed.AccessToken)
	assertOopsCode(t, err, codes.AuthTokenInvalid)
	_, err = authSvc.AuthenticateUser(ctx, login2.AccessToken)
	assertOopsCode(t, err, codes.AuthTokenInvalid)
	_, err = authSvc.RefreshToken(ctx, &types.RefreshTokenRequest{
		RefreshToken: refreshed.RefreshToken,
	})
	assertOopsCode(t, err, codes.AuthTokenInvalid)

	// Disabled account: wrong password must still return invalid credentials (no account enumeration),
	// while right password returns account disabled.
	require.NoError(t, client.User.UpdateOneID(u.ID).SetStatus(userEnt.StatusSuspended).Exec(ctx))
	_, err = authSvc.Login(ctx, &types.LoginRequest{
		Email:    "alice@example.com",
		Password: "wrongpassword1",
	})
	assertOopsCode(t, err, codes.AuthInvalidCredentials)

	_, err = authSvc.Login(ctx, &types.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	assertOopsCode(t, err, codes.AuthAccountDisabled)
}

func TestBookService_CRUDAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	client, _, bookSvc, _ := setupServices(t)

	// Empty list short-circuits cleanly.
	items, total, err := bookSvc.List(ctx, metav1.ListOptions{Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int32(0), total)

	// Create with custom ULID and verify duplicate returns BOOK_ALREADY_EXISTS.
	customID := utils.GenerateULID()
	initialPrice := 49.99
	created, err := bookSvc.Create(ctx, &types.CreateBookRequest{
		BookID:      customID,
		Title:       "Go Programming",
		Author:      "Alan Donovan",
		Description: "Classic",
		Price:       &initialPrice,
	})
	require.NoError(t, err)
	assert.Equal(t, customID, created.ID)
	require.NotNil(t, created.Price)
	assert.InDelta(t, 49.99, *created.Price, 1e-9)

	// Free book (price = 0) must be distinct from unset (nil).
	zeroPrice := 0.0
	freeBook, err := bookSvc.Create(ctx, &types.CreateBookRequest{
		Title:  "Free Go Book",
		Author: "Community",
		Price:  &zeroPrice,
	})
	require.NoError(t, err)
	require.NotNil(t, freeBook.Price)
	assert.Equal(t, 0.0, *freeBook.Price)

	_, err = bookSvc.Create(ctx, &types.CreateBookRequest{
		BookID: customID,
		Title:  "Duplicate",
		Author: "Someone",
	})
	assertOopsCode(t, err, codes.BookAlreadyExists)

	// Invalid order_by returns *validator.FieldViolations with i18n keys.
	_, _, err = bookSvc.List(ctx, metav1.ListOptions{Limit: 10, OrderBy: "unknown_col"})
	var fv *validator.FieldViolations
	require.ErrorAs(t, err, &fv)
	require.Len(t, fv.Items(), 1)
	assert.Equal(t, validator.KeyUnknownOrderByField, fv.Items()[0].Key)

	// Comma-only order_by falls back to default sort without error.
	items, total, err = bookSvc.List(ctx, metav1.ListOptions{Limit: 10, OrderBy: " , , "})
	require.NoError(t, err)
	assert.Equal(t, int32(2), total)
	assert.Len(t, items, 2)

	// Replace clears omitted optional fields (including nullable price → nil).
	replaced, err := bookSvc.Replace(ctx, customID, &types.ReplaceBookRequest{
		Book:   customID,
		Title:  "Go Programming 2nd",
		Author: "Alan Donovan",
	})
	require.NoError(t, err)
	assert.Equal(t, "Go Programming 2nd", replaced.Title)
	assert.Empty(t, replaced.Description)
	assert.Nil(t, replaced.Price)

	// Soft delete must succeed (regression test for SetDeleteTime → SetDeletedAt).
	require.NoError(t, bookSvc.Delete(ctx, customID))

	// Soft-deleted book is hidden from Get, Replace, and Delete.
	_, err = bookSvc.Get(ctx, customID)
	assertOopsCode(t, err, codes.BookNotFound)

	_, err = bookSvc.Replace(ctx, customID, &types.ReplaceBookRequest{
		Book:   customID,
		Title:  "Ghost",
		Author: "Ghost",
	})
	assertOopsCode(t, err, codes.BookNotFound)

	err = bookSvc.Delete(ctx, customID)
	assertOopsCode(t, err, codes.BookNotFound)

	// Even a direct Ent UpdateOneID must refuse to modify a soft-deleted row.
	err = client.Book.UpdateOneID(customID).SetTitle("Mutated").Exec(ctx)
	assert.True(t, ent.IsNotFound(err), "expected NotFoundError on soft-deleted row, got %v", err)
}

func TestMeService_ReplaceAndChangePassword(t *testing.T) {
	ctx := context.Background()
	_, authSvc, _, meSvc := setupServices(t)

	reg, err := authSvc.Register(ctx, &types.RegisterRequest{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "password123",
	})
	require.NoError(t, err)

	u, err := authSvc.AuthenticateUser(ctx, reg.AccessToken)
	require.NoError(t, err)

	updated, err := meSvc.Replace(ctx, u.ID, &types.ReplaceMeRequest{
		Username: "bob_new",
		Email:    "bob_new@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "bob_new", updated.Username)

	err = meSvc.ChangePassword(ctx, u.ID, &types.ChangePasswordRequest{
		OldPassword: "wrongpassword1",
		NewPassword: "newpassword123",
	})
	assertOopsCode(t, err, codes.MeInvalidPassword)

	require.NoError(t, meSvc.ChangePassword(ctx, u.ID, &types.ChangePasswordRequest{
		OldPassword: "password123",
		NewPassword: "newpassword123",
	}))

	// Changing password must revoke all active tokens.
	_, err = authSvc.AuthenticateUser(ctx, reg.AccessToken)
	assertOopsCode(t, err, codes.AuthTokenInvalid)
	_, err = authSvc.RefreshToken(ctx, &types.RefreshTokenRequest{
		RefreshToken: reg.RefreshToken,
	})
	assertOopsCode(t, err, codes.AuthTokenInvalid)
}
