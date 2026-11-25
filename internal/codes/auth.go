package codes

import "net/http"

// Auth 领域错误码。
//
// 命名规范：<领域>_<错误语义>，使用 UPPER_SNAKE_CASE。
const (
	// AuthInvalidCredentials 表示邮箱或密码错误。
	// 故意不区分"邮箱不存在"与"密码错误"，防止用户枚举攻击。
	AuthInvalidCredentials Code = "AUTH_INVALID_CREDENTIALS"

	// AuthTokenInvalid 表示 token 不合法、已过期或已撤销。
	AuthTokenInvalid Code = "AUTH_TOKEN_INVALID"

	// AuthAccountDisabled 表示账户已被停用。
	AuthAccountDisabled Code = "AUTH_ACCOUNT_DISABLED"

	// AuthEmailTaken 表示邮箱已被注册。
	AuthEmailTaken Code = "AUTH_EMAIL_TAKEN"

	// AuthUsernameTaken 表示用户名已被占用。
	AuthUsernameTaken Code = "AUTH_USERNAME_TAKEN"

	// AuthInternalError 表示认证服务内部错误。
	AuthInternalError Code = "AUTH_INTERNAL_ERROR"
)

func init() {
	Register(AuthInvalidCredentials, http.StatusUnauthorized)
	Register(AuthTokenInvalid, http.StatusUnauthorized)
	Register(AuthAccountDisabled, http.StatusForbidden)
	Register(AuthEmailTaken, http.StatusConflict)
	Register(AuthUsernameTaken, http.StatusConflict)
	Register(AuthInternalError, http.StatusInternalServerError)
}
