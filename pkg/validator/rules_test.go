package validator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestIsValidEmail(t *testing.T) {
	assert.True(t, validator.IsValidEmail("user@example.com"))
	assert.False(t, validator.IsValidEmail(""))
	assert.False(t, validator.IsValidEmail("   "))
	assert.False(t, validator.IsValidEmail("John Doe <john@example.com>"))
	assert.False(t, validator.IsValidEmail(strings.Repeat("a", 250)+"@example.com"))
}

func TestCheckPassword_BcryptMaxLength(t *testing.T) {
	var v validator.FieldViolations
	validator.CheckPassword(&v, "password", strings.Repeat("a", 72)+"1")
	require.True(t, v.HasErrors())
	assert.Equal(t, validator.KeyMaxLength, v.Items()[0].Key)
	assert.Equal(t, "72", v.Items()[0].Vars["max"])
}

func TestChangePasswordRequest_EmptyFieldsNoSpuriousMustDiffer(t *testing.T) {
	req := &types.ChangePasswordRequest{}
	err := req.Validate()
	var fv *validator.FieldViolations
	require.ErrorAs(t, err, &fv)
	for _, it := range fv.Items() {
		assert.NotEqual(t, validator.KeyMustDiffer, it.Key)
	}
}

func TestCreateBookRequest_DescriptionMaxLength(t *testing.T) {
	req := &types.CreateBookRequest{
		Title:       "Go",
		Author:      "Author",
		Description: strings.Repeat("x", 1001),
	}
	err := req.Validate()
	var fv *validator.FieldViolations
	require.ErrorAs(t, err, &fv)
	require.Len(t, fv.Items(), 1)
	assert.Equal(t, "description", fv.Items()[0].Field)
	assert.Equal(t, validator.KeyMaxLength, fv.Items()[0].Key)
}
