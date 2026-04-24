package types

import (
	"strings"
	"time"

	"github.com/liukeshao/echo-template/pkg/validator"
)

// UserInfo 用户信息（当前已登录用户的公开资料）。
// 供 GET /api/v1/me 响应使用，不含密码等敏感字段。
type UserInfo struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (r *ChangePasswordRequest) Validate() error {
	var v validator.FieldViolations
	if strings.TrimSpace(r.OldPassword) == "" {
		v.Add("old_password", validator.KeyRequired, map[string]string{"field": "old_password"})
	}
	validator.CheckPassword(&v, "new_password", r.NewPassword)
	if r.OldPassword == r.NewPassword {
		v.Add("new_password", validator.KeyMustDiffer, map[string]string{
			"field": "new_password", "other": "old_password",
		})
	}
	return v.Err()
}

// UpdateUsernameRequest 更新用户名请求
type UpdateUsernameRequest struct {
	Username string `json:"username"`
}

func (r *UpdateUsernameRequest) Validate() error {
	var v validator.FieldViolations
	validator.CheckUsername(&v, "username", r.Username)
	return v.Err()
}

// UpdateEmailRequest 更新邮箱请求
type UpdateEmailRequest struct {
	Email string `json:"email"`
}

func (r *UpdateEmailRequest) Validate() error {
	var v validator.FieldViolations
	if !validator.IsValidEmail(r.Email) {
		v.Add("email", validator.KeyEmail, map[string]string{"field": "email"})
	}
	return v.Err()
}
