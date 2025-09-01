package types

import (
	z "github.com/Oudwins/zog"

	"github.com/liukeshao/echo-template/pkg/apperrs"
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
func (i *CreateUserInput) Validate() *apperrs.Response {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	if issuesMap != nil {
		return &apperrs.Response{
			Code:   400,
			Errors: FormatIssuesAsErrorDetails(issuesMap),
		}
	}
	return nil
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
}

// Validate 验证更新用户输入
func (i *UpdateUserInput) Validate() *apperrs.Response {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	if issuesMap != nil {
		return &apperrs.Response{
			Code:   400,
			Errors: FormatIssuesAsErrorDetails(issuesMap),
		}
	}
	return nil
}

func (i *UpdateUserInput) Shape() z.Shape {

	return z.Shape{
		"Username": z.String().Min(3).Max(50).Optional(),
		"Email":    z.String().Email().Optional(),
	}
}

// ListUsersInput 获取用户列表输入
type ListUsersInput struct {
	PageInput
	Status  string `query:"status"`  // 状态筛选
	Keyword string `query:"keyword"` // 用于搜索用户名或邮箱
}

// Validate 验证获取用户列表输入
func (i *ListUsersInput) Validate() *apperrs.Response {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	if issuesMap != nil {
		return &apperrs.Response{
			Code:   400,
			Errors: FormatIssuesAsErrorDetails(issuesMap),
		}
	}
	return nil
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
