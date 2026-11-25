package validator

import (
	"net/mail"
	"slices"
	"strings"
	"unicode"
	"uuid"
)

// 校验规则码常量。既作为 rest.FieldError.Code 对外暴露，又与
// internal/i18n/messages/*.toml 中 [validation] section 的键名 1:1 对应。
const (
	CodeRequired                = "required"
	CodeMinLength               = "min_length"
	CodeMaxLength               = "max_length"
	CodeLengthRange             = "length_range"
	CodeUsernameCharset         = "username_charset"
	CodeEmail                   = "email"
	CodePasswordWeak            = "password_weak"
	CodeInteger                 = "integer"
	CodeNumber                  = "number"
	CodeBoolean                 = "boolean"
	CodeNonNegative             = "non_negative"
	CodeMinValue                = "min_value"
	CodeMaxValue                = "max_value"
	CodePattern                 = "pattern"
	CodeOneOf                   = "one_of"
	CodeInvalidType             = "invalid_type"
	CodeMustDiffer              = "must_differ"
	CodeUUID                    = "uuid"
	CodeInvalidOrderByClause    = "invalid_order_by_clause"
	CodeInvalidOrderByDirection = "invalid_order_by_direction"
	CodeUnknownOrderByField     = "unknown_order_by_field"
)

var registeredRules = []string{
	CodeRequired,
	CodeMinLength,
	CodeMaxLength,
	CodeLengthRange,
	CodeUsernameCharset,
	CodeEmail,
	CodePasswordWeak,
	CodeInteger,
	CodeNumber,
	CodeBoolean,
	CodeNonNegative,
	CodeMinValue,
	CodeMaxValue,
	CodePattern,
	CodeOneOf,
	CodeInvalidType,
	CodeMustDiffer,
	CodeUUID,
	CodeInvalidOrderByClause,
	CodeInvalidOrderByDirection,
	CodeUnknownOrderByField,
}

// RegisteredRules 返回所有内置校验规则码副本，供 i18n 在启动与测试阶段做 100% 翻译覆盖率门禁。
func RegisteredRules() []string {
	return slices.Clone(registeredRules)
}

// --- 原子级规则（供外部直接判断） ---

// IsValidUUID 验证字符串是否为 RFC 9562 标准 36 字符连字符格式 UUID。
func IsValidUUID(str string) bool {
	if len(str) != 36 {
		return false
	}
	u, err := uuid.Parse(str)
	return err == nil && strings.EqualFold(str, u.String())
}

// IsValidEmail 使用标准库解析纯邮箱地址（不接受 "Name <addr>" 展示名形式），
// 同时限制长度不超过数据库列上限（255）。空字符串视为无效。
func IsValidEmail(email string) bool {
	if strings.TrimSpace(email) == "" || len(email) > 255 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

// IsValidUsername 只允许 ASCII 字母、数字、下划线和连字符（与 TypeSpec ^[a-zA-Z0-9_-]+$ 一致）。
func IsValidUsername(s string) bool {
	for _, r := range s {
		isASCIIAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !isASCIIAlphaNum && r != '_' && r != '-' {
			return false
		}
	}
	return true
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

// --- 复合规则（直接写入 FieldViolations，减少业务层重复判断） ---

// CheckUsername 校验用户名长度（3..50，与 TypeSpec 及数据库列一致）与字符集，失败时写入 v。
func CheckUsername(v *FieldViolations, field, username string) {
	if strings.TrimSpace(username) == "" {
		v.Add(field, CodeRequired, nil)
		return
	}
	if len(username) < 3 || len(username) > 50 {
		v.Add(field, CodeLengthRange, map[string]string{
			"min": "3", "max": "50",
		})
		return
	}
	if !IsValidUsername(username) {
		v.Add(field, CodeUsernameCharset, nil)
	}
}

// CheckPassword 校验密码长度（8..72 字节，受 bcrypt 上限约束）与复杂度，失败时写入 v。
func CheckPassword(v *FieldViolations, field, password string) {
	if len(password) < 8 {
		v.Add(field, CodeMinLength, map[string]string{
			"min": "8",
		})
		return
	}
	if len(password) > 72 {
		v.Add(field, CodeMaxLength, map[string]string{
			"max": "72",
		})
		return
	}
	if !HasPasswordComplexity(password) {
		v.Add(field, CodePasswordWeak, nil)
	}
}
