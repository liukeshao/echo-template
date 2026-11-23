package types

import (
	"strings"
	"time"

	"github.com/liukeshao/echo-template/pkg/validator"
)

// User 是用户资源的对外公开表示（供 GET / PUT / PATCH /api/v1/me 返回），不含密码等敏感字段。
//
// 同时作为 PATCH "read-modify-write" 的中间 JSON 表示：
// PATCH handler 序列化该结构后应用 patch，再反序列化为 ReplaceMeRequest 写回。
//
// id / status / last_login_at / created_at / updated_at 在响应中可见但客户端不可修改 ——
// 它们不出现在 ReplaceMeRequest 中，patch 阶段对这些字段的修改会被自动丢弃。
type User struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ReplaceMeRequest 替换（PUT）当前用户资料 —— 幂等全量更新。
//
// 仅包含可修改字段（username / email）；只读字段在 PATCH 反序列化阶段自动丢弃。
type ReplaceMeRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (r *ReplaceMeRequest) Validate() error {
	var v validator.FieldViolations
	validator.CheckUsername(&v, "username", r.Username)
	if !validator.IsValidEmail(r.Email) {
		v.Add("email", validator.KeyEmail, map[string]string{"field": "email"})
	}
	return v.Err()
}

// ChangePasswordRequest 修改密码请求（自定义子资源动作）。
//
// 之所以独立于 PUT /me：
//   - 密码字段非个人资料的一部分，不应当出现在 User 响应中
//   - 修改密码需要校验旧密码，PUT /me 的全量替换语义无法表达此要求
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
	if r.OldPassword != "" && r.OldPassword == r.NewPassword {
		v.Add("new_password", validator.KeyMustDiffer, map[string]string{
			"field": "new_password", "other": "old_password",
		})
	}
	return v.Err()
}
