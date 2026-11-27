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

// 内置通用校验规则码常量。
// 均通过 RegisteredRules() 暴露，供 i18n 启动门禁保证 100% 翻译覆盖。
const (
	RuleRequired                Rule = "required"
	RuleMinLength               Rule = "min_length"
	RuleMaxLength               Rule = "max_length"
	RuleMaxBytes                Rule = "max_bytes"
	RuleEmail                   Rule = "email"
	RulePasswordWeak            Rule = "password_weak"
	RuleInteger                 Rule = "integer"
	RuleNumber                  Rule = "number"
	RuleBoolean                 Rule = "boolean"
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
	RuleEmail,
	RulePasswordWeak,
	RuleInteger,
	RuleNumber,
	RuleBoolean,
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

// RegisteredRules 返回所有内置校验规则码副本，供 i18n 在启动与测试阶段做 100% 翻译覆盖率门禁。
func RegisteredRules() []Rule {
	return slices.Clone(registeredRules)
}

// Number 约束所有可比较的 Go 整数与浮点类型，供泛型数值边界校验函数使用。
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// --- 纯原子判定函数（无副作用，供独立调用） ---

// IsValidUUID 验证字符串是否为 RFC 9562 标准 36 字符连字符格式 UUID。
func IsValidUUID(str string) bool {
	if len(str) != 36 {
		return false
	}
	u, err := uuid.Parse(str)
	return err == nil && strings.EqualFold(str, u.String())
}

// IsValidEmail 使用标准库解析纯邮箱地址（不接受 "Name <addr>" 展示名形式；长度约束由 TypeSpec @maxLength 控制）。
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

// --- 声明式字段检查方法（返回 true 表示通过，false 表示已记录校验错误，便于使用 && 做同字段短路） ---

// Required 校验字符串去除首尾空白后非空。
func (v *FieldViolations) Required(field, val string) bool {
	if strings.TrimSpace(val) == "" {
		v.Add(field, RuleRequired)
		return false
	}
	return true
}

// MinLength 校验字符串 Unicode 字符数不少于 min。
func (v *FieldViolations) MinLength(field, val string, min int) bool {
	if utf8.RuneCountInString(val) < min {
		v.Add(field, RuleMinLength, P("min", strconv.Itoa(min)))
		return false
	}
	return true
}

// MaxLength 校验字符串 Unicode 字符数不超过 max。
func (v *FieldViolations) MaxLength(field, val string, max int) bool {
	if utf8.RuneCountInString(val) > max {
		v.Add(field, RuleMaxLength, P("max", strconv.Itoa(max)))
		return false
	}
	return true
}

// MaxBytes 校验字符串 UTF-8 字节长度不超过 max（例如 bcrypt 72 字节上限）。
func (v *FieldViolations) MaxBytes(field, val string, max int) bool {
	if len(val) > max {
		v.Add(field, RuleMaxBytes, P("max", strconv.Itoa(max)))
		return false
	}
	return true
}

// Pattern 校验字符串是否匹配预编译正则 re。
func (v *FieldViolations) Pattern(field, val string, re *regexp.Regexp) bool {
	if !re.MatchString(val) {
		v.Add(field, RulePattern, P("pattern", re.String()))
		return false
	}
	return true
}

// UUID 校验字符串为合法 RFC 9562 UUID。
func (v *FieldViolations) UUID(field, val string) bool {
	if !IsValidUUID(val) {
		v.Add(field, RuleUUID)
		return false
	}
	return true
}

// Email 校验字符串为合法纯邮箱地址。
func (v *FieldViolations) Email(field, val string) bool {
	if !IsValidEmail(val) {
		v.Add(field, RuleEmail)
		return false
	}
	return true
}

// PasswordComplexity 校验密码至少包含一个字母和一个数字。
func (v *FieldViolations) PasswordComplexity(field, val string) bool {
	if !HasPasswordComplexity(val) {
		v.Add(field, RulePasswordWeak)
		return false
	}
	return true
}

// MustDiffer 校验非空字段 val 必须不同于另一字段 otherVal（用于跨字段校验，如新旧密码不能相同）。
func (v *FieldViolations) MustDiffer(field, val, otherField, otherVal string) bool {
	if val != "" && val == otherVal {
		v.Add(field, RuleMustDiffer, P("other", otherField))
		return false
	}
	return true
}

// --- 泛型边界与枚举检查函数 ---

// Min 校验 val >= min；当 min == 0 时使用语义更精准的 RuleNonNegative。
func Min[T Number](v *FieldViolations, field string, val, min T) bool {
	if val < min {
		if min == 0 {
			v.Add(field, RuleNonNegative, P("min", "0"))
		} else {
			v.Add(field, RuleMinValue, P("min", fmt.Sprint(min)))
		}
		return false
	}
	return true
}

// Max 校验 val <= max。
func Max[T Number](v *FieldViolations, field string, val, max T) bool {
	if val > max {
		v.Add(field, RuleMaxValue, P("max", fmt.Sprint(max)))
		return false
	}
	return true
}

// OneOf 校验枚举或字符串字面量值是否位于 allowed 集合中。
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
