package validator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestIsValidEmail(t *testing.T) {
	t.Parallel()
	assert.True(t, validator.IsValidEmail("user@example.com"))
	assert.False(t, validator.IsValidEmail(""))
	assert.False(t, validator.IsValidEmail("   "))
	assert.False(t, validator.IsValidEmail("John Doe <john@example.com>"))
	assert.False(t, validator.IsValidEmail(strings.Repeat("a", 250)+"@example.com"))
}

func TestCheckUsername(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantKey string
	}{
		{"valid", "alice_01-x", ""},
		{"empty", "   ", validator.KeyRequired},
		{"too short", "ab", validator.KeyLengthRange},
		{"too long", strings.Repeat("a", 33), validator.KeyLengthRange},
		{"invalid charset", "alice@bad", validator.KeyCharset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v validator.FieldViolations
			validator.CheckUsername(&v, "username", tc.in)
			if tc.wantKey == "" {
				assert.NoError(t, v.Err())
			} else {
				require.True(t, v.HasErrors())
				assert.Equal(t, tc.wantKey, v.Items()[0].Key)
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantKey string
	}{
		{"valid", "password123", ""},
		{"too short", "pass12", validator.KeyMinLength},
		{"exceeds bcrypt 72 bytes", strings.Repeat("a", 72) + "1", validator.KeyMaxLength},
		{"letters only", "abcdefghij", validator.KeyPasswordWeak},
		{"digits only", "1234567890", validator.KeyPasswordWeak},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v validator.FieldViolations
			validator.CheckPassword(&v, "password", tc.in)
			if tc.wantKey == "" {
				assert.NoError(t, v.Err())
			} else {
				require.True(t, v.HasErrors())
				assert.Equal(t, tc.wantKey, v.Items()[0].Key)
			}
		})
	}
}

func TestFieldViolations_ErrorFormattingAndCopy(t *testing.T) {
	t.Parallel()
	var v validator.FieldViolations
	assert.Nil(t, v.Items())
	assert.NoError(t, v.Err())

	vars := map[string]string{"field": "title"}
	v.Add("title", validator.KeyRequired, vars)
	v.Add("", validator.KeyMustDiffer, nil)
	vars["field"] = "mutated" // caller mutation must not affect stored violation

	items := v.Items()
	require.Len(t, items, 2)
	assert.Equal(t, "title", items[0].Vars["field"])
	assert.Contains(t, v.Error(), "title: validation.required{field=title}")
	assert.Contains(t, v.Error(), "validation.must_differ")
}
