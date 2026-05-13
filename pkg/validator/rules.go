package validator

import (
	"net/mail"
	"strings"
	"unicode"
)

// 翻译键常量。集中定义避免字符串散落各处；任何新规则须同时新增 key 与翻译。
//
// 与 pkg/i18n/messages/*.toml 中 [validation] section 的键一一对应。
const (
	KeyRequired     = "validation.required"
	KeyMinLength    = "validation.min_length"
	KeyMaxLength    = "validation.max_length"
	KeyLengthRange  = "validation.length_range"
	KeyCharset      = "validation.charset"
	KeyEmail        = "validation.email"
	KeyPasswordWeak = "validation.password_weak"
	KeyNonNegative  = "validation.non_negative"
	KeyMustDiffer   = "validation.must_differ"
	KeyULID         = "validation.ulid"
)

// --- 原子级规则（供外部直接判断） ---

// IsValidEmail 使用标准库解析邮箱地址，空字符串视为无效。
func IsValidEmail(email string) bool {
	if strings.TrimSpace(email) == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

// IsValidUsername 只允许字母、数字、下划线和连字符。
func IsValidUsername(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
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

// --- 复合规则（直接写入 FieldViolations，减少 types 里的重复判断） ---

// usernameAllowedCharset 是 CheckUsername 在 charset 规则下填入模板的 {charset} 值。
// 仍为英文短语；如需对 charset 描述本地化，可进一步拆分多个 key。
const usernameAllowedCharset = "letters, digits, underscores and hyphens"

// CheckUsername 校验用户名长度与字符集，失败时写入 v。
func CheckUsername(v *FieldViolations, field, username string) {
	if strings.TrimSpace(username) == "" {
		v.Add(field, KeyRequired, map[string]string{"field": field})
		return
	}
	if len(username) < 3 || len(username) > 32 {
		v.Add(field, KeyLengthRange, map[string]string{
			"field": field, "min": "3", "max": "32",
		})
		return
	}
	if !IsValidUsername(username) {
		v.Add(field, KeyCharset, map[string]string{
			"field":   field,
			"charset": usernameAllowedCharset,
		})
	}
}

// CheckPassword 校验密码长度与复杂度，失败时写入 v。
func CheckPassword(v *FieldViolations, field, password string) {
	if len(password) < 8 {
		v.Add(field, KeyMinLength, map[string]string{
			"field": field, "min": "8",
		})
		return
	}
	if !HasPasswordComplexity(password) {
		v.Add(field, KeyPasswordWeak, map[string]string{"field": field})
	}
}
