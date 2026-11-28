package validator

import (
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	"uuid"
)

const (
	RuleRequired                Rule = "required"
	RuleMinLength               Rule = "min_length"
	RuleMaxLength               Rule = "max_length"
	RuleMaxBytes                Rule = "max_bytes"
	RuleMinItems                Rule = "min_items"
	RuleMaxItems                Rule = "max_items"
	RuleEmail                   Rule = "email"
	RulePasswordWeak            Rule = "password_weak"
	RuleInteger                 Rule = "integer"
	RuleNumber                  Rule = "number"
	RuleBoolean                 Rule = "boolean"
	RuleDateTime                Rule = "datetime"
	RuleNonNegative             Rule = "non_negative"
	RuleMinValue                Rule = "min_value"
	RuleMaxValue                Rule = "max_value"
	RulePattern                 Rule = "pattern"
	RuleOneOf                   Rule = "one_of"
	RuleInvalidType             Rule = "invalid_type"
	RuleMustDiffer              Rule = "must_differ"
	RuleUUID                    Rule = "uuid"
	RuleInvalidOrderByClause    Rule = "invalid_order_by_clause"
	RuleInvalidOrderByDirection Rule = "invalid_order_by_direction"
	RuleUnknownOrderByField     Rule = "unknown_order_by_field"
)

var registeredRules = []Rule{
	RuleRequired,
	RuleMinLength,
	RuleMaxLength,
	RuleMaxBytes,
	RuleMinItems,
	RuleMaxItems,
	RuleEmail,
	RulePasswordWeak,
	RuleInteger,
	RuleNumber,
	RuleBoolean,
	RuleDateTime,
	RuleNonNegative,
	RuleMinValue,
	RuleMaxValue,
	RulePattern,
	RuleOneOf,
	RuleInvalidType,
	RuleMustDiffer,
	RuleUUID,
	RuleInvalidOrderByClause,
	RuleInvalidOrderByDirection,
	RuleUnknownOrderByField,
}

func RegisteredRules() []Rule {
	return slices.Clone(registeredRules)
}

type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

func IsValidUUID(str string) bool {
	if len(str) != 36 {
		return false
	}
	u, err := uuid.Parse(str)
	return err == nil && strings.EqualFold(str, u.String())
}

// IsValidEmail 仅接受纯邮箱地址（拒绝 "Name <addr>" 展示名形式）。
func IsValidEmail(email string) bool {
	if strings.TrimSpace(email) == "" {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

// HasPasswordComplexity 要求密码至少包含一个字母和一个数字。
func HasPasswordComplexity(s string) bool {
	var hasLetter, hasDigit bool
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
		if hasLetter && hasDigit {
			return true
		}
	}
	return false
}

func (v *FieldViolations) Required(field, val string) bool {
	if strings.TrimSpace(val) == "" {
		v.Add(field, RuleRequired)
		return false
	}
	return true
}

func (v *FieldViolations) MinLength(field, val string, minLen int) bool {
	if utf8.RuneCountInString(val) < minLen {
		v.Add(field, RuleMinLength, P("min", strconv.Itoa(minLen)))
		return false
	}
	return true
}

func (v *FieldViolations) MaxLength(field, val string, maxLen int) bool {
	if utf8.RuneCountInString(val) > maxLen {
		v.Add(field, RuleMaxLength, P("max", strconv.Itoa(maxLen)))
		return false
	}
	return true
}

func (v *FieldViolations) MaxBytes(field, val string, maxBytes int) bool {
	if len(val) > maxBytes {
		v.Add(field, RuleMaxBytes, P("max", strconv.Itoa(maxBytes)))
		return false
	}
	return true
}

func (v *FieldViolations) MinItems(field string, count, minItems int) bool {
	if count < minItems {
		v.Add(field, RuleMinItems, P("min", strconv.Itoa(minItems)))
		return false
	}
	return true
}

func (v *FieldViolations) MaxItems(field string, count, maxItems int) bool {
	if count > maxItems {
		v.Add(field, RuleMaxItems, P("max", strconv.Itoa(maxItems)))
		return false
	}
	return true
}

func (v *FieldViolations) Pattern(field, val string, re *regexp.Regexp) bool {
	if !re.MatchString(val) {
		v.Add(field, RulePattern, P("pattern", re.String()))
		return false
	}
	return true
}

func (v *FieldViolations) UUID(field, val string) bool {
	if !IsValidUUID(val) {
		v.Add(field, RuleUUID)
		return false
	}
	return true
}

func (v *FieldViolations) Email(field, val string) bool {
	if !IsValidEmail(val) {
		v.Add(field, RuleEmail)
		return false
	}
	return true
}

func (v *FieldViolations) PasswordComplexity(field, val string) bool {
	if !HasPasswordComplexity(val) {
		v.Add(field, RulePasswordWeak)
		return false
	}
	return true
}

func (v *FieldViolations) MustDiffer(field, val, otherField, otherVal string) bool {
	if val != "" && val == otherVal {
		v.Add(field, RuleMustDiffer, P("other", otherField))
		return false
	}
	return true
}

func Min[T Number](v *FieldViolations, field string, val, minVal T) bool {
	if val < minVal {
		if minVal == 0 {
			v.Add(field, RuleNonNegative, P("min", "0"))
		} else {
			v.Add(field, RuleMinValue, P("min", fmt.Sprint(minVal)))
		}
		return false
	}
	return true
}

func Max[T Number](v *FieldViolations, field string, val, maxVal T) bool {
	if val > maxVal {
		v.Add(field, RuleMaxValue, P("max", fmt.Sprint(maxVal)))
		return false
	}
	return true
}

func OneOf[T ~string](v *FieldViolations, field string, val T, allowed ...T) bool {
	if slices.Contains(allowed, val) {
		return true
	}
	strs := make([]string, len(allowed))
	for i, a := range allowed {
		strs[i] = string(a)
	}
	v.Add(field, RuleOneOf, P("allowed", strings.Join(strs, ", ")))
	return false
}
