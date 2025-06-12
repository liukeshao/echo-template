package types

import (
	"time"

	z "github.com/Oudwins/zog"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// CreateRoleInput 创建角色输入
type CreateRoleInput struct {
	Name        string `json:"name"`        // 角色名称
	Code        string `json:"code"`        // 角色代码
	Description string `json:"description"` // 角色描述
	Status      string `json:"status"`      // 角色状态
	SortOrder   int    `json:"sort_order"`  // 排序顺序
}

// Validate 验证创建角色输入
func (i *CreateRoleInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Name": z.String().
			Min(1, z.Message("角色名称不能为空")).
			Max(50, z.Message("角色名称长度不能超过50个字符")).
			Required(z.Message("角色名称是必填项")),
		"Code": z.String().
			Min(1, z.Message("角色代码不能为空")).
			Max(50, z.Message("角色代码长度不能超过50个字符")).
			Required(z.Message("角色代码是必填项")),
		"Description": z.String().
			Max(255, z.Message("角色描述长度不能超过255个字符")).
			Optional(),
		"Status": z.String().
			OneOf([]string{"active", "inactive"}, z.Message("角色状态只能是active或inactive")).
			Default("active"),
		"SortOrder": z.Int().
			GTE(0, z.Message("排序顺序不能小于0")).
			Default(0),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// UpdateRoleInput 更新角色输入
type UpdateRoleInput struct {
	Name        *string `json:"name,omitempty"`        // 角色名称
	Description *string `json:"description,omitempty"` // 角色描述
	Status      *string `json:"status,omitempty"`      // 角色状态
	SortOrder   *int    `json:"sort_order,omitempty"`  // 排序顺序
}

// Validate 验证更新角色输入
func (i *UpdateRoleInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Name": z.String().
			Min(1, z.Message("角色名称不能为空")).
			Max(50, z.Message("角色名称长度不能超过50个字符")).
			Optional(),
		"Description": z.String().
			Max(255, z.Message("角色描述长度不能超过255个字符")).
			Optional(),
		"Status": z.String().
			OneOf([]string{"active", "inactive"}, z.Message("角色状态只能是active或inactive")).
			Optional(),
		"SortOrder": z.Int().
			GTE(0, z.Message("排序顺序不能小于0")).
			Optional(),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// AssignRoleInput 分配角色输入
type AssignRoleInput struct {
	UserID    string     `json:"user_id"`              // 用户ID
	RoleIDs   []string   `json:"role_ids"`             // 角色ID列表
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // 过期时间
	Remark    string     `json:"remark,omitempty"`     // 备注
}

// Validate 验证分配角色输入
func (i *AssignRoleInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"UserID": z.String().
			Min(26, z.Message("用户ID格式不正确")).
			Max(26, z.Message("用户ID格式不正确")).
			Required(z.Message("用户ID是必填项")),
		"RoleIDs": z.Slice(z.String().
			Min(26, z.Message("角色ID格式不正确")).
			Max(26, z.Message("角色ID格式不正确"))).
			Min(1, z.Message("至少需要选择一个角色")).
			Required(z.Message("角色ID列表是必填项")),
		"Remark": z.String().
			Max(255, z.Message("备注长度不能超过255个字符")).
			Optional(),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// RevokeRoleInput 撤销角色输入
type RevokeRoleInput struct {
	UserID  string   `json:"user_id"`  // 用户ID
	RoleIDs []string `json:"role_ids"` // 角色ID列表
}

// Validate 验证撤销角色输入
func (i *RevokeRoleInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"UserID": z.String().
			Min(26, z.Message("用户ID格式不正确")).
			Max(26, z.Message("用户ID格式不正确")).
			Required(z.Message("用户ID是必填项")),
		"RoleIDs": z.Slice(z.String().
			Min(26, z.Message("角色ID格式不正确")).
			Max(26, z.Message("角色ID格式不正确"))).
			Min(1, z.Message("至少需要选择一个角色")).
			Required(z.Message("角色ID列表是必填项")),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// ListRolesInput 获取角色列表输入
type ListRolesInput struct {
	Page     int    `query:"page"`      // 页码
	PageSize int    `query:"page_size"` // 每页数量
	Status   string `query:"status"`    // 状态过滤
	Search   string `query:"search"`    // 搜索关键词
}

// Validate 验证获取角色列表输入
func (i *ListRolesInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Page": z.Int().
			GTE(1, z.Message("页码不能小于1")).
			Default(1),
		"page_size": z.Int().
			GTE(1, z.Message("每页数量不能小于1")).
			LTE(100, z.Message("每页数量不能大于100")).
			Default(20),
		"Status": z.String().
			OneOf([]string{"", "active", "inactive"}, z.Message("状态值无效")).
			Optional(),
		"Search": z.String().
			Max(100, z.Message("搜索关键词长度不能超过100个字符")).
			Optional(),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// RoleOutput 角色输出
type RoleOutput struct {
	ID          string    `json:"id"`          // 角色ID
	Name        string    `json:"name"`        // 角色名称
	Code        string    `json:"code"`        // 角色代码
	Description *string   `json:"description"` // 角色描述
	Status      string    `json:"status"`      // 角色状态
	IsSystem    bool      `json:"is_system"`   // 是否系统角色
	SortOrder   int       `json:"sort_order"`  // 排序顺序
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`  // 更新时间
	Permissions []string  `json:"permissions"` // 权限代码列表
}

// ListRolesOutput 角色列表输出
type ListRolesOutput struct {
	Roles      []*RoleOutput `json:"roles"`       // 角色列表
	Total      int64         `json:"total"`       // 总数
	Page       int           `json:"page"`        // 当前页码
	PageSize   int           `json:"page_size"`   // 每页数量
	TotalPages int           `json:"total_pages"` // 总页数
}

// UserRoleOutput 用户角色输出
type UserRoleOutput struct {
	ID        string     `json:"id"`         // 用户角色关联ID
	UserID    string     `json:"user_id"`    // 用户ID
	RoleID    string     `json:"role_id"`    // 角色ID
	RoleName  string     `json:"role_name"`  // 角色名称
	RoleCode  string     `json:"role_code"`  // 角色代码
	GrantedBy *string    `json:"granted_by"` // 授权者ID
	GrantedAt time.Time  `json:"granted_at"` // 授权时间
	ExpiresAt *time.Time `json:"expires_at"` // 过期时间
	Status    string     `json:"status"`     // 状态
	Remark    *string    `json:"remark"`     // 备注
}

// ListUserRolesOutput 用户角色列表输出
type ListUserRolesOutput struct {
	UserRoles  []*UserRoleOutput `json:"user_roles"`  // 用户角色列表
	Total      int64             `json:"total"`       // 总数
	Page       int               `json:"page"`        // 当前页码
	PageSize   int               `json:"page_size"`   // 每页数量
	TotalPages int               `json:"total_pages"` // 总页数
}

// AssignRoleMenusInput 为角色分配菜单输入
type AssignRoleMenusInput struct {
	RoleID  string   `json:"role_id"`  // 角色ID
	MenuIDs []string `json:"menu_ids"` // 菜单ID列表
}

// Validate 验证为角色分配菜单输入
func (i *AssignRoleMenusInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"RoleID": z.String().
			Min(26, z.Message("角色ID格式不正确")).
			Max(26, z.Message("角色ID格式不正确")).
			Required(z.Message("角色ID是必填项")),
		"MenuIDs": z.Slice(z.String().
			Min(26, z.Message("菜单ID格式不正确")).
			Max(26, z.Message("菜单ID格式不正确"))).
			Min(1, z.Message("至少需要选择一个菜单")).
			Required(z.Message("菜单ID列表是必填项")),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// GetUserMenusInput 获取用户菜单输入
type GetUserMenusInput struct {
	UserID   string `query:"user_id"`   // 用户ID
	TreeMode bool   `query:"tree_mode"` // 是否返回树形结构
	OnlyMenu bool   `query:"only_menu"` // 是否只返回菜单类型（排除按钮）
}

// Validate 验证获取用户菜单输入
func (i *GetUserMenusInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"UserID": z.String().
			Min(26, z.Message("用户ID格式不正确")).
			Max(26, z.Message("用户ID格式不正确")).
			Required(z.Message("用户ID是必填项")),
		"TreeMode": z.Bool().Default(true),
		"OnlyMenu": z.Bool().Default(true),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// RoleMenuOutput 角色菜单输出
type RoleMenuOutput struct {
	RoleID   string      `json:"role_id"`   // 角色ID
	RoleName string      `json:"role_name"` // 角色名称
	Menus    []*MenuInfo `json:"menus"`     // 菜单列表
}

// UserMenuOutput 用户菜单输出
type UserMenuOutput struct {
	UserID string      `json:"user_id"` // 用户ID
	Menus  []*MenuInfo `json:"menus"`   // 用户可访问的菜单列表
}

// MenuPermissionInfo 菜单权限信息
type MenuPermissionInfo struct {
	*MenuInfo
	Permissions []string `json:"permissions"` // 菜单关联的权限列表
}

// RoleMenuPermissionOutput 角色菜单权限输出
type RoleMenuPermissionOutput struct {
	RoleID      string                `json:"role_id"`     // 角色ID
	RoleName    string                `json:"role_name"`   // 角色名称
	Menus       []*MenuPermissionInfo `json:"menus"`       // 菜单列表（包含权限）
	Permissions []string              `json:"permissions"` // 角色的所有权限
}
