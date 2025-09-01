package types

import (
	z "github.com/Oudwins/zog"

	"github.com/liukeshao/echo-template/pkg/apperrs"
)

// UpdateMeInput 更新用户输入
type UpdateMeInput struct {
	Username *string `json:"username,omitempty"` // 用户名
	Email    *string `json:"email,omitempty"`    // 邮箱
}

// Validate 验证更新用户输入
func (i *UpdateMeInput) Validate() *apperrs.Response {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	if issuesMap != nil {
		return &apperrs.Response{
			Code:   400,
			Errors: FormatIssuesAsErrorDetails(issuesMap),
		}
	}
	return nil
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
func (i *ChangePasswordInput) Validate() *apperrs.Response {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	if issuesMap != nil {
		return &apperrs.Response{
			Code:   400,
			Errors: FormatIssuesAsErrorDetails(issuesMap),
		}
	}
	return nil
}

func (i *ChangePasswordInput) Shape() z.Shape {

	return z.Shape{
		"OldPassword": z.String().Required(),
		"NewPassword": z.String().Min(8).Required(),
	}
}

// UpdateUsernameInput 更新用户名输入
type UpdateUsernameInput struct {
	Username string `json:"username"` // 用户名
}

// Validate 验证更新用户名输入
func (i *UpdateUsernameInput) Validate() *apperrs.Response {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	if issuesMap != nil {
		return &apperrs.Response{
			Code:   400,
			Errors: FormatIssuesAsErrorDetails(issuesMap),
		}
	}
	return nil
}

func (i *UpdateUsernameInput) Shape() z.Shape {
	return z.Shape{
		"Username": z.String().Min(3).Max(50).Required(),
	}
}

// UpdateEmailInput 更新邮箱输入
type UpdateEmailInput struct {
	Email string `json:"email"` // 邮箱
}

// Validate 验证更新邮箱输入
func (i *UpdateEmailInput) Validate() *apperrs.Response {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	if issuesMap != nil {
		return &apperrs.Response{
			Code:   400,
			Errors: FormatIssuesAsErrorDetails(issuesMap),
		}
	}
	return nil
}

func (i *UpdateEmailInput) Shape() z.Shape {
	return z.Shape{
		"Email": z.String().Email().Required(),
	}
}
