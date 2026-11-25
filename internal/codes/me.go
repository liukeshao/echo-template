package codes

import "net/http"

// Me 领域错误码（已登录用户的个人信息操作）。
//
// 命名规范：<领域>_<错误语义>，使用 UPPER_SNAKE_CASE。
const (
	// MeUsernameTaken 表示用户名已被其他用户占用。
	MeUsernameTaken Code = "ME_USERNAME_TAKEN"

	// MeEmailTaken 表示邮箱已被其他用户使用。
	MeEmailTaken Code = "ME_EMAIL_TAKEN"

	// MeInvalidPassword 表示当前密码验证失败。
	MeInvalidPassword Code = "ME_INVALID_PASSWORD"

	// MeInternalError 表示用户信息服务内部错误。
	MeInternalError Code = "ME_INTERNAL_ERROR"
)

func init() {
	Register(MeUsernameTaken, http.StatusConflict)
	Register(MeEmailTaken, http.StatusConflict)
	Register(MeInvalidPassword, http.StatusBadRequest)
	Register(MeInternalError, http.StatusInternalServerError)
}
