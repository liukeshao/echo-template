package api_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/utils"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestValidate_AuthRequests(t *testing.T) {
	t.Parallel()

	assert.NoError(t, (&api.RegisterRequest{Username: "alice", Email: "alice@example.com", Password: "password123"}).Validate())
	assert.Error(t, (&api.RegisterRequest{}).Validate())

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
		assert.NotEqual(t, validator.KeyMustDiffer, it.Key)
	}

	// Cross-field validation provided by me_validate.go (validateExtra hook).
	err = (&api.ChangePasswordRequest{OldPassword: "samepassword1", NewPassword: "samepassword1"}).Validate()
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.KeyMustDiffer, fv.Items()[0].Key)
}

func TestValidate_BookAndListRequests(t *testing.T) {
	t.Parallel()

	neg := -1.0
	pos := 10.0
	validID := utils.GenerateULID()

	assert.NoError(t, (&api.CreateBookRequest{
		BookID: validID,
		Title:  "Go",
		Author: "Alan",
		Price:  &pos,
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

	assert.NoError(t, (&api.ReplaceBookRequest{Book: validID, Title: "Go", Author: "Alan"}).Validate())
	assert.Error(t, (&api.ReplaceBookRequest{Book: "bad-id", Title: "Go", Author: "Alan"}).Validate())
	assert.Error(t, (&api.ReplaceBookRequest{Book: validID}).Validate())

	assert.NoError(t, (&api.GetBookRequest{Book: validID}).Validate())
	assert.Error(t, (&api.GetBookRequest{Book: "bad-id"}).Validate())

	assert.NoError(t, (&api.DeleteBookRequest{Book: validID}).Validate())
	assert.Error(t, (&api.DeleteBookRequest{Book: "bad-id"}).Validate())

	assert.NoError(t, (&api.ListBooksRequest{Page: 1, PageSize: 20}).Validate())
	assert.Error(t, (&api.ListBooksRequest{Page: -1, PageSize: -5}).Validate())
}

func TestProblem_ErrorString(t *testing.T) {
	t.Parallel()

	p := api.Problem{
		Status: http.StatusNotFound,
		Code:   "BOOK_NOT_FOUND",
		Detail: "Book 01TEST was not found.",
	}
	assert.Equal(t, "http 404 BOOK_NOT_FOUND: Book 01TEST was not found.", p.Error())
}
