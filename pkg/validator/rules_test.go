package validator_test

import (
	"errors"
	"regexp"
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
	assert.False(t, validator.IsValidEmail("not-an-email"))
}

func TestFieldViolations_StringAndSemanticChecks(t *testing.T) {
	t.Parallel()
	usernameRe := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	var v validator.FieldViolations
	assert.True(t, v.Required("title", "Go"))
	assert.False(t, v.Required("title", "   "))
	assert.True(t, v.MinLength("name", "你好呀", 3))
	assert.False(t, v.MinLength("name", "你好", 3))
	assert.True(t, v.MaxLength("name", "你好", 2))
	assert.False(t, v.MaxLength("name", "你好呀", 2))
	assert.True(t, v.MaxBytes("pw", strings.Repeat("a", 72), 72))
	assert.False(t, v.MaxBytes("pw", strings.Repeat("中", 25), 72)) // 75 bytes > 72
	assert.True(t, v.Pattern("username", "alice_01-x", usernameRe))
	assert.False(t, v.Pattern("username", "alice@bad", usernameRe))
	assert.True(t, v.UUID("id", uuid.NewV7().String()))
	assert.False(t, v.UUID("id", "bad"))
	assert.True(t, v.Email("email", "a@b.com"))
	assert.False(t, v.Email("email", "bad"))
	assert.True(t, v.PasswordComplexity("pw", "pass1234"))
	assert.False(t, v.PasswordComplexity("pw", "abcdefgh"))
	assert.False(t, v.PasswordComplexity("pw", "12345678"))
	assert.True(t, v.MustDiffer("new_pw", "", "old_pw", "")) // empty does not trigger false positive
	assert.True(t, v.MustDiffer("new_pw", "new1", "old_pw", "old1"))
	assert.False(t, v.MustDiffer("new_pw", "same1", "old_pw", "same1"))

	items := v.Items()
	require.Len(t, items, 10)
	assert.Equal(t, validator.RuleRequired, items[0].Code)
	assert.Equal(t, validator.RuleMinLength, items[1].Code)
	assert.Equal(t, "3", items[1].Metadata["min"])
	assert.Equal(t, validator.RuleMaxLength, items[2].Code)
	assert.Equal(t, "2", items[2].Metadata["max"])
	assert.Equal(t, validator.RuleMaxBytes, items[3].Code)
	assert.Equal(t, "72", items[3].Metadata["max"])
	assert.Equal(t, validator.RulePattern, items[4].Code)
	assert.Equal(t, `^[a-zA-Z0-9_-]+$`, items[4].Metadata["pattern"])
	assert.Equal(t, validator.RuleUUID, items[5].Code)
	assert.Equal(t, validator.RuleEmail, items[6].Code)
	assert.Equal(t, validator.RulePasswordWeak, items[7].Code)
	assert.Equal(t, validator.RulePasswordWeak, items[8].Code)
	assert.Equal(t, validator.RuleMustDiffer, items[9].Code)
	assert.Equal(t, "old_pw", items[9].Metadata["other"])
}

func TestGenericNumericAndEnumChecks(t *testing.T) {
	t.Parallel()
	type status string

	var v validator.FieldViolations
	assert.True(t, validator.Min(&v, "page", int32(0), 0))
	assert.False(t, validator.Min(&v, "page", int32(-1), 0))
	assert.False(t, validator.Min(&v, "age", 15, 18))
	assert.True(t, validator.Max(&v, "price", 99.5, 100.0))
	assert.False(t, validator.Max(&v, "price", 101.5, 100.0))
	assert.True(t, validator.OneOf(&v, "status", status("active"), "active", "inactive"))
	assert.False(t, validator.OneOf(&v, "status", status("deleted"), "active", "inactive"))

	items := v.Items()
	require.Len(t, items, 4)
	assert.Equal(t, validator.RuleNonNegative, items[0].Code)
	assert.Equal(t, "0", items[0].Metadata["min"])
	assert.Equal(t, validator.RuleMinValue, items[1].Code)
	assert.Equal(t, "18", items[1].Metadata["min"])
	assert.Equal(t, validator.RuleMaxValue, items[2].Code)
	assert.Equal(t, "100", items[2].Metadata["max"])
	assert.Equal(t, validator.RuleOneOf, items[3].Code)
	assert.Equal(t, "active, inactive", items[3].Metadata["allowed"])
}

func TestParseOrderBy(t *testing.T) {
	t.Parallel()
	allowed := []string{"title", "price", "created_at"}

	terms, err := validator.ParseOrderBy("order_by", "   ", allowed...)
	require.NoError(t, err)
	assert.Nil(t, terms)

	terms, err = validator.ParseOrderBy("order_by", " , , ", allowed...)
	require.NoError(t, err)
	assert.Empty(t, terms)

	terms, err = validator.ParseOrderBy("order_by", "PRICE DESC, title", allowed...)
	require.NoError(t, err)
	assert.Equal(t, []validator.OrderTerm{
		{Field: "price", Desc: true},
		{Field: "title", Desc: false},
	}, terms)

	// Collects all errors in a single pass.
	_, err = validator.ParseOrderBy("order_by", "title asc extra, title asc, unknown", allowed...)
	fv, ok := errors.AsType[*validator.FieldViolations](err)
	require.True(t, ok)
	items := fv.Items()
	require.Len(t, items, 3)
	assert.Equal(t, validator.RuleInvalidOrderByClause, items[0].Code)
	assert.Equal(t, "title asc extra", items[0].Metadata["clause"])
	assert.Equal(t, validator.RuleInvalidOrderByDirection, items[1].Code)
	assert.Equal(t, "asc", items[1].Metadata["direction"])
	assert.Equal(t, validator.RuleUnknownOrderByField, items[2].Code)
	assert.Equal(t, "unknown", items[2].Metadata["value"])
	assert.Equal(t, "title, price, created_at", items[2].Metadata["allowed"])
}

func TestFieldViolations_ErrorFormattingAndImmutability(t *testing.T) {
	t.Parallel()
	var v validator.FieldViolations
	assert.Nil(t, v.Items())
	assert.NoError(t, v.Err())

	v.Add("title", validator.RuleMaxLength, validator.P("max", "200"))
	v.Add("", validator.RuleMustDiffer)
	v.Add("ignored", validator.RuleRequired, validator.P("", "skip"))

	items := v.Items()
	require.Len(t, items, 3)
	assert.Equal(t, "200", items[0].Metadata["max"])
	assert.Nil(t, items[2].Metadata)

	// Mutating returned slice metadata must not mutate internal violations.
	items[0].Metadata["max"] = "999"
	assert.Equal(t, "200", v.Items()[0].Metadata["max"])

	assert.Contains(t, v.Error(), "title: max_length{max=200}")
	assert.Contains(t, v.Error(), "must_differ")
	assert.Equal(t, "required", validator.RuleRequired.Key())
	assert.Equal(t, "required", validator.RuleRequired.String())
	assert.Contains(t, validator.RegisteredRules(), validator.RuleRequired)
	assert.Contains(t, validator.RegisteredRules(), validator.RuleMaxBytes)
	assert.Contains(t, validator.RegisteredRules(), validator.RuleUnknownOrderByField)
}
