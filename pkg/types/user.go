package types

import (
	z "github.com/Oudwins/zog"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// CreateUserInput 创建用户输入
type CreateUserInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Status   string `json:"status,omitempty"` // 可选，默认为active
}

// Validate 验证创建用户输入
func (i *CreateUserInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Username": z.String().Min(3, z.Message("用户名长度不能小于3")).Max(50, z.Message("用户名长度不能大于50")).Required(z.Message("用户名不能为空")),
		"Email":    z.String().Email(z.Message("邮箱格式不正确")).Required(z.Message("邮箱不能为空")),
		"Password": z.String().Min(8, z.Message("密码长度不能小于8")).Required(z.Message("密码不能为空")),
		"Status":   z.String().OneOf([]string{"active", "inactive", "suspended"}, z.Message("状态必须是 active, inactive 或 suspended")).Optional(),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// UpdateUserInput 更新用户输入
type UpdateUserInput struct {
	Username *string `json:"username,omitempty"`
	Email    *string `json:"email,omitempty"`
	Status   *string `json:"status,omitempty"`
}

// Validate 验证更新用户输入
func (i *UpdateUserInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Username": z.String().Min(3, z.Message("用户名长度不能小于3")).Max(50, z.Message("用户名长度不能大于50")).Optional(),
		"Email":    z.String().Email(z.Message("邮箱格式不正确")).Optional(),
		"Password": z.String().Min(8, z.Message("密码长度不能小于8")).Optional(),
		"Status":   z.String().OneOf([]string{"active", "inactive", "suspended"}, z.Message("状态必须是 active, inactive 或 suspended")).Optional(),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// ChangePasswordInput 修改密码输入
type ChangePasswordInput struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// Validate 验证修改密码输入
func (i *ChangePasswordInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"OldPassword": z.String().Required(z.Message("旧密码不能为空")),
		"NewPassword": z.String().Min(8, z.Message("新密码长度不能小于8")).Required(z.Message("新密码不能为空")),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// ListUsersInput 获取用户列表输入
type ListUsersInput struct {
	Page     int    `query:"page"`
	PageSize int    `query:"page_size"`
	Status   string `query:"status"`
	Keyword  string `query:"keyword"` // 用于搜索用户名或邮箱
}

// Validate 验证获取用户列表输入
func (i *ListUsersInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"page":      z.Int().GTE(1).Default(1),
		"page_size": z.Int().GTE(1).LTE(100).Default(20),
		"Status":    z.String().OneOf([]string{"active", "inactive", "suspended"}, z.Message("状态必须是 active, inactive 或 suspended")).Optional(),
		"Keyword":   z.String().Optional(),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// UserOutput 用户输出
type UserOutput struct {
	*UserInfo
}

// ListUsersOutput 获取用户列表输出
type ListUsersOutput struct {
	Users      []*UserInfo `json:"users"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// UserStatsOutput 用户统计输出
type UserStatsOutput struct {
	TotalUsers     int64 `json:"total_users"`
	ActiveUsers    int64 `json:"active_users"`
	InactiveUsers  int64 `json:"inactive_users"`
	SuspendedUsers int64 `json:"suspended_users"`
}
