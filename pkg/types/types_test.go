package types_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestValidate_AuthRequests(t *testing.T) {
	t.Parallel()

	assert.NoError(t, (&types.RegisterRequest{Username: "alice", Email: "alice@example.com", Password: "password123"}).Validate())
	assert.Error(t, (&types.RegisterRequest{}).Validate())

	assert.NoError(t, (&types.LoginRequest{Email: "alice@example.com", Password: "x"}).Validate())
	assert.Error(t, (&types.LoginRequest{}).Validate())

	assert.NoError(t, (&types.RefreshTokenRequest{RefreshToken: "tok"}).Validate())
	assert.Error(t, (&types.RefreshTokenRequest{RefreshToken: "  "}).Validate())
}

func TestValidate_MeRequests(t *testing.T) {
	t.Parallel()

	assert.NoError(t, (&types.ReplaceMeRequest{UserSpec: types.UserSpec{Username: "alice", Email: "alice@example.com"}}).Validate())
	assert.Error(t, (&types.ReplaceMeRequest{UserSpec: types.UserSpec{Username: "a", Email: "bad"}}).Validate())

	assert.NoError(t, (&types.ChangePasswordRequest{OldPassword: "oldpassword1", NewPassword: "newpassword1"}).Validate())

	err := (&types.ChangePasswordRequest{}).Validate()
	var fv *validator.FieldViolations
	require.ErrorAs(t, err, &fv)
	for _, it := range fv.Items() {
		assert.NotEqual(t, validator.KeyMustDiffer, it.Key)
	}

	err = (&types.ChangePasswordRequest{OldPassword: "samepassword1", NewPassword: "samepassword1"}).Validate()
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.KeyMustDiffer, fv.Items()[0].Key)
}

func TestValidate_BookAndListRequests(t *testing.T) {
	t.Parallel()

	neg := -1.0
	pos := 10.0

	assert.NoError(t, (&types.CreateBookRequest{
		BookID:   utils.GenerateULID(),
		BookSpec: types.BookSpec{Title: "Go", Author: "Alan", Price: &pos},
	}).Validate())
	assert.Error(t, (&types.CreateBookRequest{
		BookID:   "bad-id",
		BookSpec: types.BookSpec{Title: "Go", Author: "Alan"},
	}).Validate())
	assert.Error(t, (&types.CreateBookRequest{
		BookSpec: types.BookSpec{
			Title:       strings.Repeat("t", 201),
			Author:      strings.Repeat("a", 101),
			Description: strings.Repeat("x", 1001),
			Price:       &neg,
		},
	}).Validate())

	assert.NoError(t, (&types.ReplaceBookRequest{BookSpec: types.BookSpec{Title: "Go", Author: "Alan"}}).Validate())
	assert.Error(t, (&types.ReplaceBookRequest{}).Validate())

	assert.NoError(t, (&types.ListRequest{Page: 1, PageSize: 20}).Validate())
	assert.Error(t, (&types.ListRequest{Page: -1, PageSize: -5}).Validate())

	// NewListResponse ensures nil slice serializes as [] instead of null.
	empty := types.NewListResponse[types.Book](nil, 1, 50, 0)
	b, err := json.Marshal(empty)
	require.NoError(t, err)
	assert.JSONEq(t, `{"items":[],"page":1,"page_size":50,"total_size":0}`, string(b))
}
