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
	Username            string   `json:"username"`                        // 用户名
	Email               string   `json:"email"`                           // 邮箱
	Password            string   `json:"password"`                        // 密码
	RealName            *string  `json:"real_name,omitempty"`             // 真实姓名
	Phone               *string  `json:"phone,omitempty"`                 // 手机号
	Department          *string  `json:"department,omitempty"`            // 所属部门
	Position            *string  `json:"position,omitempty"`              // 岗位
	Roles               []string `json:"roles,omitempty"`                 // 用户角色列表
	Status              string   `json:"status,omitempty"`                // 状态，默认为active
	ForceChangePassword bool     `json:"force_change_password,omitempty"` // 是否强制修改密码
	AllowMultiLogin     bool     `json:"allow_multi_login,omitempty"`     // 是否允许多端登录
}

// Validate 验证创建用户输入
func (i *CreateUserInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *CreateUserInput) Shape() z.Shape {
	return z.Shape{
		"Username":            z.String().Min(3).Max(50).Required(),
		"Email":               z.String().Email().Required(),
		"Password":            z.String().Min(8).Required(),
		"RealName":            z.String().Max(100).Optional(),
		"Phone":               z.String().Max(20).Optional(),
		"Department":          z.String().Max(100).Optional(),
		"Position":            z.String().Max(100).Optional(),
		"Roles":               z.Slice(z.String()).Optional(),
		"Status":              z.String().OneOf(UserStatuses()).Optional(),
		"ForceChangePassword": z.Bool().Optional(),
		"AllowMultiLogin":     z.Bool().Optional(),
	}
}

// UpdateUserInput 更新用户输入（管理员用）
type UpdateUserInput struct {
	Username            *string  `json:"username,omitempty"`              // 用户名
	Email               *string  `json:"email,omitempty"`                 // 邮箱
	RealName            *string  `json:"real_name,omitempty"`             // 真实姓名
	Phone               *string  `json:"phone,omitempty"`                 // 手机号
	Department          *string  `json:"department,omitempty"`            // 所属部门
	Position            *string  `json:"position,omitempty"`              // 岗位
	Roles               []string `json:"roles,omitempty"`                 // 用户角色列表
	Status              *string  `json:"status,omitempty"`                // 状态
	ForceChangePassword *bool    `json:"force_change_password,omitempty"` // 是否强制修改密码
	AllowMultiLogin     *bool    `json:"allow_multi_login,omitempty"`     // 是否允许多端登录
}

// Validate 验证更新用户输入
func (i *UpdateUserInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *UpdateUserInput) Shape() z.Shape {
	return z.Shape{
		"Username":            z.String().Min(3).Max(50).Optional(),
		"Email":               z.String().Email().Optional(),
		"RealName":            z.String().Max(100).Optional(),
		"Phone":               z.String().Max(20).Optional(),
		"Department":          z.String().Max(100).Optional(),
		"Position":            z.String().Max(100).Optional(),
		"Roles":               z.Slice(z.String()).Optional(),
		"Status":              z.String().OneOf(UserStatuses()).Optional(),
		"ForceChangePassword": z.Bool().Optional(),
		"AllowMultiLogin":     z.Bool().Optional(),
	}
}

// ListUsersInput 获取用户列表输入
type ListUsersInput struct {
	PageInput
	Status     string `query:"status"`     // 状态筛选
	Department string `query:"department"` // 部门筛选
	Position   string `query:"position"`   // 岗位筛选
	Role       string `query:"role"`       // 角色筛选
	Keyword    string `query:"keyword"`    // 用于搜索用户名、邮箱、真实姓名、手机号
}

// Validate 验证获取用户列表输入
func (i *ListUsersInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *ListUsersInput) Shape() z.Shape {
	return z.Shape{
		"PageInput":  z.Ptr(z.Struct(i.PageInput.Shape())),
		"Status":     z.String().OneOf(UserStatuses()).Optional(),
		"Department": z.String().Optional(),
		"Position":   z.String().Optional(),
		"Role":       z.String().Optional(),
		"Keyword":    z.String().Optional(),
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

// ResetPasswordInput 重置密码输入
type ResetPasswordInput struct {
	NewPassword         string `json:"new_password"`          // 新密码
	ForceChangePassword bool   `json:"force_change_password"` // 是否强制用户下次登录时修改密码
}

// Validate 验证重置密码输入
func (i *ResetPasswordInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *ResetPasswordInput) Shape() z.Shape {
	return z.Shape{
		"NewPassword":         z.String().Min(8).Required(),
		"ForceChangePassword": z.Bool().Optional(),
	}
}

// SetUserStatusInput 设置用户状态输入
type SetUserStatusInput struct {
	Status string `json:"status"` // 用户状态
}

// Validate 验证设置用户状态输入
func (i *SetUserStatusInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *SetUserStatusInput) Shape() z.Shape {
	return z.Shape{
		"Status": z.String().OneOf(UserStatuses()).Required(),
	}
}

// BatchOperationInput 批量操作输入
type BatchOperationInput struct {
	UserIDs []string `json:"user_ids"` // 用户ID列表
}

// Validate 验证批量操作输入
func (i *BatchOperationInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *BatchOperationInput) Shape() z.Shape {
	return z.Shape{
		"UserIDs": z.Slice(z.String()).Min(1).Required(),
	}
}

// BatchUpdateStatusInput 批量更新状态输入
type BatchUpdateStatusInput struct {
	UserIDs []string `json:"user_ids"` // 用户ID列表
	Status  string   `json:"status"`   // 新状态
}

// Validate 验证批量更新状态输入
func (i *BatchUpdateStatusInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *BatchUpdateStatusInput) Shape() z.Shape {
	return z.Shape{
		"UserIDs": z.Slice(z.String()).Min(1).Required(),
		"Status":  z.String().OneOf(UserStatuses()).Required(),
	}
}

// UserStatsOutput 用户统计输出
type UserStatsOutput struct {
	TotalUsers      int64            `json:"total_users"`      // 总用户数
	ActiveUsers     int64            `json:"active_users"`     // 活跃用户数
	InactiveUsers   int64            `json:"inactive_users"`   // 非活跃用户数
	SuspendedUsers  int64            `json:"suspended_users"`  // 停用用户数
	StatusBreakdown map[string]int64 `json:"status_breakdown"` // 状态分布
	DepartmentStats []DepartmentStat `json:"department_stats"` // 部门统计
}

// DepartmentStat 部门统计
type DepartmentStat struct {
	Department string `json:"department"` // 部门名称
	UserCount  int64  `json:"user_count"` // 用户数量
}
