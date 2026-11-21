package types_test

import (
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

	assert.NoError(t, (&types.ReplaceMeRequest{Username: "alice", Email: "alice@example.com"}).Validate())
	assert.Error(t, (&types.ReplaceMeRequest{Username: "a", Email: "bad"}).Validate())

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

func TestValidate_BookRequests(t *testing.T) {
	t.Parallel()

	neg := -1.0
	pos := 10.0

	assert.NoError(t, (&types.CreateBookRequest{BookID: utils.GenerateULID(), Title: "Go", Author: "Alan", Price: &pos}).Validate())
	assert.Error(t, (&types.CreateBookRequest{BookID: "bad-id", Title: "Go", Author: "Alan"}).Validate())
	assert.Error(t, (&types.CreateBookRequest{Title: strings.Repeat("t", 201), Author: strings.Repeat("a", 101), Description: strings.Repeat("x", 1001), Price: &neg}).Validate())

	assert.NoError(t, (&types.ReplaceBookRequest{Title: "Go", Author: "Alan"}).Validate())
	assert.Error(t, (&types.ReplaceBookRequest{}).Validate())

	assert.NoError(t, (&types.ListBooksRequest{Page: 1, PageSize: 20}).Validate())
	assert.Error(t, (&types.ListBooksRequest{Page: -1, PageSize: -5}).Validate())
}
