package validator

import (
	"net/mail"
	"strings"
	"unicode"
	"uuid"
)

// 翻译键常量。集中定义避免字符串散落各处；任何新规则须同时新增 key 与翻译。
//
// 与 pkg/i18n/messages/*.toml 中 [validation] section 的键一一对应。
const (
	KeyRequired                = "validation.required"
	KeyMinLength               = "validation.min_length"
	KeyMaxLength               = "validation.max_length"
	KeyLengthRange             = "validation.length_range"
	KeyUsernameCharset         = "validation.username_charset"
	KeyEmail                   = "validation.email"
	KeyPasswordWeak            = "validation.password_weak"
	KeyInteger                 = "validation.integer"
	KeyNumber                  = "validation.number"
	KeyBoolean                 = "validation.boolean"
	KeyNonNegative             = "validation.non_negative"
	KeyMinValue                = "validation.min_value"
	KeyMaxValue                = "validation.max_value"
	KeyPattern                 = "validation.pattern"
	KeyOneOf                   = "validation.one_of"
	KeyInvalidType             = "validation.invalid_type"
	KeyMustDiffer              = "validation.must_differ"
	KeyUUID                    = "validation.uuid"
	KeyInvalidOrderByClause    = "validation.invalid_order_by_clause"
	KeyInvalidOrderByDirection = "validation.invalid_order_by_direction"
	KeyUnknownOrderByField     = "validation.unknown_order_by_field"
)

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
		v.Add(field, KeyRequired, map[string]string{"field": field})
		return
	}
	if len(username) < 3 || len(username) > 50 {
		v.Add(field, KeyLengthRange, map[string]string{
			"field": field, "min": "3", "max": "50",
		})
		return
	}
	if !IsValidUsername(username) {
		v.Add(field, KeyUsernameCharset, map[string]string{"field": field})
	}
}

// CheckPassword 校验密码长度（8..72 字节，受 bcrypt 上限约束）与复杂度，失败时写入 v。
func CheckPassword(v *FieldViolations, field, password string) {
	if len(password) < 8 {
		v.Add(field, KeyMinLength, map[string]string{
			"field": field, "min": "8",
		})
		return
	}
	if len(password) > 72 {
		v.Add(field, KeyMaxLength, map[string]string{
			"field": field, "max": "72",
		})
		return
	}
	if !HasPasswordComplexity(password) {
		v.Add(field, KeyPasswordWeak, map[string]string{"field": field})
	}
}
