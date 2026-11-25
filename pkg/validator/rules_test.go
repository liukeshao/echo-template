package validator_test

import (
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestIsValidUUID(t *testing.T) {
	t.Parallel()
	id1 := uuid.NewV7().String()
	id2 := uuid.NewV7().String()

	assert.Len(t, id1, 36)
	assert.Less(t, id1, id2, "UUID v7 should be monotonically ordered")
	assert.True(t, validator.IsValidUUID(id1))
	assert.False(t, validator.IsValidUUID(""))
	assert.False(t, validator.IsValidUUID("not-a-uuid"))
	assert.False(t, validator.IsValidUUID("urn:uuid:"+id1))
	assert.False(t, validator.IsValidUUID("f81d4fae7dec11d0a76500a0c91e6bf6"))
}

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
		name     string
		in       string
		wantCode string
	}{
		{"valid", "alice_01-x", ""},
		{"valid max 50", strings.Repeat("a", 50), ""},
		{"empty", "   ", validator.CodeRequired},
		{"too short", "ab", validator.CodeLengthRange},
		{"too long", strings.Repeat("a", 51), validator.CodeLengthRange},
		{"invalid charset", "alice@bad", validator.CodeUsernameCharset},
		{"non-ascii rejected", "中文用户", validator.CodeUsernameCharset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v validator.FieldViolations
			validator.CheckUsername(&v, "username", tc.in)
			if tc.wantCode == "" {
				assert.NoError(t, v.Err())
			} else {
				require.True(t, v.HasErrors())
				assert.Equal(t, tc.wantCode, v.Items()[0].Code)
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		in       string
		wantCode string
	}{
		{"valid", "password123", ""},
		{"too short", "pass12", validator.CodeMinLength},
		{"exceeds bcrypt 72 bytes", strings.Repeat("a", 72) + "1", validator.CodeMaxLength},
		{"letters only", "abcdefghij", validator.CodePasswordWeak},
		{"digits only", "1234567890", validator.CodePasswordWeak},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v validator.FieldViolations
			validator.CheckPassword(&v, "password", tc.in)
			if tc.wantCode == "" {
				assert.NoError(t, v.Err())
			} else {
				require.True(t, v.HasErrors())
				assert.Equal(t, tc.wantCode, v.Items()[0].Code)
			}
		})
	}
}

func TestFieldViolations_ErrorFormattingAndCopy(t *testing.T) {
	t.Parallel()
	var v validator.FieldViolations
	assert.Nil(t, v.Items())
	assert.NoError(t, v.Err())

	meta := map[string]string{"max": "200"}
	v.Add("title", validator.CodeMaxLength, meta)
	v.Add("", validator.CodeMustDiffer, nil)
	meta["max"] = "999" // caller mutation must not affect stored violation

	items := v.Items()
	require.Len(t, items, 2)
	assert.Equal(t, "200", items[0].Metadata["max"])
	assert.Contains(t, v.Error(), "title: max_length{max=200}")
	assert.Contains(t, v.Error(), "must_differ")

	assert.Contains(t, validator.RegisteredRules(), validator.CodeRequired)
	assert.Contains(t, validator.RegisteredRules(), validator.CodeUnknownOrderByField)
}
