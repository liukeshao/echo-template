package api_test

import (
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/go-rest-template/pkg/api"
	"github.com/liukeshao/go-rest-template/pkg/validator"
)

func TestValidate_AuthRequests(t *testing.T) {
	t.Parallel()

	assert.NoError(t, (&api.RegisterRequest{Username: "alice", Email: "alice@example.com", Password: "password123"}).Validate())
	assert.Error(t, (&api.RegisterRequest{}).Validate())

	// Username constraints come directly from TypeSpec (@minLength(3), @maxLength(50), @pattern).
	var fv *validator.FieldViolations
	err := (&api.RegisterRequest{Username: "ab", Email: "alice@example.com", Password: "password123"}).Validate()
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.RuleMinLength, fv.Items()[0].Code)

	err = (&api.RegisterRequest{Username: "alice@bad", Email: "alice@example.com", Password: "password123"}).Validate()
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.RulePattern, fv.Items()[0].Code)

	// Email @maxLength(255) enforced via TypeSpec scalar.
	err = (&api.RegisterRequest{Username: "alice", Email: strings.Repeat("a", 250) + "@example.com", Password: "password123"}).Validate()
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.RuleMaxLength, fv.Items()[0].Code)

	// Password bcrypt 72-byte limit enforced via MaxBytes + complexity check.
	err = (&api.RegisterRequest{Username: "alice", Email: "alice@example.com", Password: strings.Repeat("a", 72) + "1"}).Validate()
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.RuleMaxBytes, fv.Items()[0].Code)

	err = (&api.RegisterRequest{Username: "alice", Email: "alice@example.com", Password: "onlyletters"}).Validate()
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.RulePasswordWeak, fv.Items()[0].Code)

	assert.NoError(t, (&api.LoginRequest{Email: "alice@example.com", Password: "x"}).Validate())
	assert.Error(t, (&api.LoginRequest{}).Validate())

	assert.NoError(t, (&api.RefreshTokenRequest{RefreshToken: "tok"}).Validate())
	assert.Error(t, (&api.RefreshTokenRequest{RefreshToken: "  "}).Validate())
}

func TestValidate_MeRequests(t *testing.T) {
	t.Parallel()

	assert.NoError(t, (&api.ReplaceMeRequest{Username: "alice", Email: "alice@example.com"}).Validate())
	assert.Error(t, (&api.ReplaceMeRequest{Username: "a", Email: "bad"}).Validate())

	assert.NoError(t, (&api.ChangePasswordRequest{OldPassword: "oldpassword1", NewPassword: "newpassword1"}).Validate())

	err := (&api.ChangePasswordRequest{}).Validate()
	var fv *validator.FieldViolations
	require.ErrorAs(t, err, &fv)
	for _, it := range fv.Items() {
		assert.NotEqual(t, validator.RuleMustDiffer, it.Code)
	}

	// Cross-field validation provided by me_validate.go (validateExtra hook).
	err = (&api.ChangePasswordRequest{OldPassword: "samepassword1", NewPassword: "samepassword1"}).Validate()
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.RuleMustDiffer, fv.Items()[0].Code)
	assert.Equal(t, "old_password", fv.Items()[0].Metadata["other"])
}

func TestValidate_BookAndListRequests(t *testing.T) {
	t.Parallel()

	neg := -1.0
	pos := 10.0
	validID := uuid.NewV7().String()

	assert.NoError(t, (&api.CreateBookRequest{
		BookID: validID,
		Title:  strings.Repeat("书", 200),
		Author: "Alan",
		Price:  &pos,
	}).Validate())
	assert.Error(t, (&api.CreateBookRequest{
		Title:  strings.Repeat("书", 201),
		Author: "Alan",
	}).Validate())
	assert.Error(t, (&api.CreateBookRequest{
		BookID: "bad-id",
		Title:  "Go",
		Author: "Alan",
	}).Validate())
	assert.Error(t, (&api.CreateBookRequest{
		Title:       strings.Repeat("t", 201),
		Author:      strings.Repeat("a", 101),
		Description: strings.Repeat("x", 1001),
		Price:       &neg,
	}).Validate())

	assert.NoError(t, (&api.ReplaceBookRequest{BookID: validID, Title: "Go", Author: "Alan"}).Validate())
	assert.Error(t, (&api.ReplaceBookRequest{BookID: "bad-id", Title: "Go", Author: "Alan"}).Validate())
	assert.Error(t, (&api.ReplaceBookRequest{BookID: validID}).Validate())

	assert.NoError(t, (&api.GetBookRequest{BookID: validID}).Validate())
	assert.Error(t, (&api.GetBookRequest{BookID: "bad-id"}).Validate())

	assert.NoError(t, (&api.DeleteBookRequest{BookID: validID}).Validate())
	assert.Error(t, (&api.DeleteBookRequest{BookID: "bad-id"}).Validate())

	assert.NoError(t, (&api.ListBooksRequest{Page: 1, PageSize: 20}).Validate())
	assert.Error(t, (&api.ListBooksRequest{Page: -1, PageSize: -5}).Validate())
}
