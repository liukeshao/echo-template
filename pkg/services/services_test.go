package services_test

import (
	"context"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/ent"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/codes"
	metav1 "github.com/liukeshao/echo-template/pkg/meta/v1"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/testutil"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestAuthService_FullLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*services.AuthService](env.Injector)

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
	testutil.AssertOopsCode(t, err, codes.AuthEmailTaken)

	_, err = authSvc.Register(ctx, &types.RegisterRequest{
		Username: "alice",
		Email:    "alice2@example.com",
		Password: "password123",
	})
	testutil.AssertOopsCode(t, err, codes.AuthUsernameTaken)

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
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)

	// Logout revokes all active access and refresh tokens for the user.
	require.NoError(t, authSvc.Logout(ctx, u.ID))
	_, err = authSvc.AuthenticateUser(ctx, refreshed.AccessToken)
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)
	_, err = authSvc.AuthenticateUser(ctx, login2.AccessToken)
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)
	_, err = authSvc.RefreshToken(ctx, &types.RefreshTokenRequest{
		RefreshToken: refreshed.RefreshToken,
	})
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)

	// Disabled account: wrong password must still return invalid credentials (no account enumeration),
	// while right password returns account disabled.
	require.NoError(t, env.ORM().User.UpdateOneID(u.ID).SetStatus(userEnt.StatusSuspended).Exec(ctx))
	_, err = authSvc.Login(ctx, &types.LoginRequest{
		Email:    "alice@example.com",
		Password: "wrongpassword1",
	})
	testutil.AssertOopsCode(t, err, codes.AuthInvalidCredentials)

	_, err = authSvc.Login(ctx, &types.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	testutil.AssertOopsCode(t, err, codes.AuthAccountDisabled)
}

func TestBookService_CRUDAndSoftDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	bookSvc := do.MustInvoke[*services.BookService](env.Injector)

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
	testutil.AssertOopsCode(t, err, codes.BookAlreadyExists)

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
	testutil.AssertOopsCode(t, err, codes.BookNotFound)

	_, err = bookSvc.Replace(ctx, customID, &types.ReplaceBookRequest{
		Book:   customID,
		Title:  "Ghost",
		Author: "Ghost",
	})
	testutil.AssertOopsCode(t, err, codes.BookNotFound)

	err = bookSvc.Delete(ctx, customID)
	testutil.AssertOopsCode(t, err, codes.BookNotFound)

	// Even a direct Ent UpdateOneID must refuse to modify a soft-deleted row.
	err = env.ORM().Book.UpdateOneID(customID).SetTitle("Mutated").Exec(ctx)
	assert.True(t, ent.IsNotFound(err), "expected NotFoundError on soft-deleted row, got %v", err)
}

func TestBookService_ListFilterAndOrderBy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	bookSvc := do.MustInvoke[*services.BookService](env.Injector)

	p1, p2 := 10.0, 50.0
	_, err := bookSvc.Create(ctx, &types.CreateBookRequest{Title: "Alpha Go", Author: "Alice", Price: &p1})
	require.NoError(t, err)
	_, err = bookSvc.Create(ctx, &types.CreateBookRequest{Title: "Beta Rust", Author: "Bob", Price: &p2})
	require.NoError(t, err)

	// Filter by title contains and price range.
	items, total, err := bookSvc.List(ctx, metav1.ListOptions{
		Limit:  10,
		Filter: `title : "go" AND price <= 20`,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "Alpha Go", items[0].Title)

	// All supported order_by fields.
	for _, ob := range []string{"title", "author desc", "price desc, title", "created_at", "updated_at desc", "published_at"} {
		_, _, err := bookSvc.List(ctx, metav1.ListOptions{Limit: 10, OrderBy: ob})
		require.NoError(t, err, "order_by=%s", ob)
	}

	// Invalid order_by clause & direction return *validator.FieldViolations.
	var fv *validator.FieldViolations
	_, _, err = bookSvc.List(ctx, metav1.ListOptions{Limit: 10, OrderBy: "title asc extra"})
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.KeyInvalidOrderByClause, fv.Items()[0].Key)

	_, _, err = bookSvc.List(ctx, metav1.ListOptions{Limit: 10, OrderBy: "title asc"})
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.KeyInvalidOrderByDirection, fv.Items()[0].Key)

	// Invalid filter syntax & unknown filter field return VALIDATION_INVALID.
	_, _, err = bookSvc.List(ctx, metav1.ListOptions{Limit: 10, Filter: `title =`})
	testutil.AssertOopsCode(t, err, codes.ValidationInvalid)

	_, _, err = bookSvc.List(ctx, metav1.ListOptions{Limit: 10, Filter: `bogus = "x"`})
	testutil.AssertOopsCode(t, err, codes.ValidationInvalid)
}

func TestAuthService_TokenTypeMismatchAndErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*services.AuthService](env.Injector)
	tok := env.MustAuthUser("tokuser", "tokuser@example.com", "password123")

	// Passing refresh token to AuthenticateUser must be rejected.
	_, err := authSvc.AuthenticateUser(ctx, tok.RefreshToken)
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)

	// Passing access token to RefreshToken must be rejected.
	_, err = authSvc.RefreshToken(ctx, &types.RefreshTokenRequest{RefreshToken: tok.AccessToken})
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)

	// Unknown email login returns AuthInvalidCredentials.
	_, err = authSvc.Login(ctx, &types.LoginRequest{Email: "nobody@example.com", Password: "password123"})
	testutil.AssertOopsCode(t, err, codes.AuthInvalidCredentials)
}

func TestMeService_ReplaceAndChangePassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	authSvc := do.MustInvoke[*services.AuthService](env.Injector)
	meSvc := do.MustInvoke[*services.MeService](env.Injector)

	reg := env.MustAuthUser("bob", "bob@example.com", "password123")
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
	testutil.AssertOopsCode(t, err, codes.MeInvalidPassword)

	require.NoError(t, meSvc.ChangePassword(ctx, u.ID, &types.ChangePasswordRequest{
		OldPassword: "password123",
		NewPassword: "newpassword123",
	}))

	// Changing password must revoke all active tokens.
	_, err = authSvc.AuthenticateUser(ctx, reg.AccessToken)
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)
	_, err = authSvc.RefreshToken(ctx, &types.RefreshTokenRequest{
		RefreshToken: reg.RefreshToken,
	})
	testutil.AssertOopsCode(t, err, codes.AuthTokenInvalid)
}
