package types

import (
	z "github.com/Oudwins/zog"
)

// 用户状态常量
const (
	UserStatusActive    = "active"
	UserStatusInactive  = "inactive"
	UserStatusSuspended = "suspended"
)

// UserStatuses 返回所有用户状态值
func UserStatuses() []string {
	return []string{UserStatusActive, UserStatusInactive, UserStatusSuspended}
}

// CreateUserInput 创建用户输入
type CreateUserInput struct {
	Username string `json:"username"`         // 用户名
	Email    string `json:"email"`            // 邮箱
	Password string `json:"password"`         // 密码
	Status   string `json:"status,omitempty"` // 可选，默认为active
}

// Validate 验证创建用户输入
func (i *CreateUserInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *CreateUserInput) Shape() z.Shape {

	return z.Shape{
		"Username": z.String().Min(3).Max(50).Required(),
		"Email":    z.String().Email().Required(),
		"Password": z.String().Min(8).Required(),
		"Status":   z.String().OneOf(UserStatuses()).Optional(),
	}
}

// UpdateUserInput 更新用户输入
type UpdateUserInput struct {
	Username *string `json:"username,omitempty"` // 用户名
	Email    *string `json:"email,omitempty"`    // 邮箱
	Status   *string `json:"status,omitempty"`   // 状态
}

// Validate 验证更新用户输入
func (i *UpdateUserInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *UpdateUserInput) Shape() z.Shape {

	return z.Shape{
		"Username": z.String().Min(3).Max(50).Optional(),
		"Email":    z.String().Email().Optional(),
		"Status":   z.String().OneOf(UserStatuses()).Optional(),
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

// ListUsersInput 获取用户列表输入
type ListUsersInput struct {
	PageInput
	Status  string `query:"status"`  // 状态筛选
	Keyword string `query:"keyword"` // 用于搜索用户名或邮箱
}

// Validate 验证获取用户列表输入
func (i *ListUsersInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *ListUsersInput) Shape() z.Shape {
	return z.Shape{
		"PageInput": z.Ptr(z.Struct(i.PageInput.Shape())),
		"Status":    z.String().OneOf(UserStatuses()).Optional(),
		"Keyword":   z.String().Optional(),
	}
}

// UserOutput 用户输出
type UserOutput struct {
	*UserInfo
}

// ListUsersOutput 获取用户列表输出
type ListUsersOutput struct {
	Users []*UserInfo `json:"users"` // 用户列表
	PageOutput
}
