package types

import (
	z "github.com/Oudwins/zog"
)

// UpdateMeInput 更新用户输入
type UpdateMeInput struct {
	Username *string `json:"username,omitempty"` // 用户名
	Email    *string `json:"email,omitempty"`    // 邮箱
}

// Validate 验证更新用户输入
func (i *UpdateMeInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *UpdateMeInput) Shape() z.Shape {
	return z.Shape{
		"Username": z.String().Min(3).Max(50).Optional(),
		"Email":    z.String().Email().Optional(),
	}
}

// ChangePasswordInput 修改密码输入
type ChangePasswordInput struct {
	OldPassword string `json:"old_password"` // 原密码
	NewPassword string `json:"new_password"` // 新密码
}

// Validate 验证修改密码输入
func (i *ChangePasswordInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *ChangePasswordInput) Shape() z.Shape {

	return z.Shape{
		"OldPassword": z.String().Required(),
		"NewPassword": z.String().Min(8).Required(),
	}
}
