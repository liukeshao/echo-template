package types

import (
	z "github.com/Oudwins/zog"
)

// 角色状态常量
const (
	RoleStatusEnabled  = "enabled"
	RoleStatusDisabled = "disabled"
)

// RoleStatuses 返回所有角色状态值
func RoleStatuses() []string {
	return []string{RoleStatusEnabled, RoleStatusDisabled}
}

// 数据权限范围常量
const (
	DataScopeAll     = "all"      // 全部数据权限
	DataScopeDeptSub = "dept_sub" // 本部门及以下数据权限
	DataScopeDept    = "dept"     // 本部门数据权限
	DataScopeSelf    = "self"     // 本人数据权限
	DataScopeCustom  = "custom"   // 自定义数据权限
)

// DataScopes 返回所有数据权限范围值
func DataScopes() []string {
	return []string{DataScopeAll, DataScopeDeptSub, DataScopeDept, DataScopeSelf, DataScopeCustom}
}

// CreateRoleInput 创建角色输入
type CreateRoleInput struct {
	Name        string   `json:"name"`                  // 角色名称
	Code        string   `json:"code"`                  // 角色编码
	Description *string  `json:"description,omitempty"` // 角色描述
	Status      string   `json:"status,omitempty"`      // 状态，默认为enabled
	DataScope   string   `json:"data_scope,omitempty"`  // 数据权限范围，默认为all
	DeptIds     []string `json:"dept_ids,omitempty"`    // 自定义部门权限ID列表
	MenuIds     []string `json:"menu_ids,omitempty"`    // 菜单权限ID列表
	SortOrder   int      `json:"sort_order,omitempty"`  // 排序顺序
	Remark      *string  `json:"remark,omitempty"`      // 角色备注
}

// Validate 验证创建角色输入
func (i *CreateRoleInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *CreateRoleInput) Shape() z.Shape {
	return z.Shape{
		"Name":        z.String().Min(1).Max(100).Required(),
		"Code":        z.String().Min(1).Max(50).Required(),
		"Description": z.String().Max(500).Optional(),
		"Status":      z.String().OneOf(RoleStatuses()).Optional(),
		"DataScope":   z.String().OneOf(DataScopes()).Optional(),
		"DeptIds":     z.Slice(z.String()).Optional(),
		"MenuIds":     z.Slice(z.String()).Optional(),
		"SortOrder":   z.Int().Optional(),
		"Remark":      z.String().Max(500).Optional(),
	}
}

// UpdateRoleInput 更新角色输入
type UpdateRoleInput struct {
	Name        *string  `json:"name,omitempty"`        // 角色名称
	Code        *string  `json:"code,omitempty"`        // 角色编码
	Description *string  `json:"description,omitempty"` // 角色描述
	Status      *string  `json:"status,omitempty"`      // 状态
	DataScope   *string  `json:"data_scope,omitempty"`  // 数据权限范围
	DeptIds     []string `json:"dept_ids,omitempty"`    // 自定义部门权限ID列表
	MenuIds     []string `json:"menu_ids,omitempty"`    // 菜单权限ID列表
	SortOrder   *int     `json:"sort_order,omitempty"`  // 排序顺序
	Remark      *string  `json:"remark,omitempty"`      // 角色备注
}

// Validate 验证更新角色输入
func (i *UpdateRoleInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *UpdateRoleInput) Shape() z.Shape {
	return z.Shape{
		"Name":        z.String().Min(1).Max(100).Optional(),
		"Code":        z.String().Min(1).Max(50).Optional(),
		"Description": z.String().Max(500).Optional(),
		"Status":      z.String().OneOf(RoleStatuses()).Optional(),
		"DataScope":   z.String().OneOf(DataScopes()).Optional(),
		"DeptIds":     z.Slice(z.String()).Optional(),
		"MenuIds":     z.Slice(z.String()).Optional(),
		"SortOrder":   z.Int().Optional(),
		"Remark":      z.String().Max(500).Optional(),
	}
}

// ListRolesInput 获取角色列表输入
type ListRolesInput struct {
	PageInput
	Status    string `query:"status"`     // 状态筛选
	DataScope string `query:"data_scope"` // 数据权限范围筛选
	Keyword   string `query:"keyword"`    // 用于搜索角色名称、角色编码
}

// Validate 验证获取角色列表输入
func (i *ListRolesInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *ListRolesInput) Shape() z.Shape {
	return z.Shape{
		"PageInput": z.Ptr(z.Struct(i.PageInput.Shape())),
		"Status":    z.String().OneOf(RoleStatuses()).Optional(),
		"DataScope": z.String().OneOf(DataScopes()).Optional(),
		"Keyword":   z.String().Optional(),
	}
}

// RoleInfo 角色信息
type RoleInfo struct {
	ID          string   `json:"id"`          // 角色ID
	Name        string   `json:"name"`        // 角色名称
	Code        string   `json:"code"`        // 角色编码
	Description *string  `json:"description"` // 角色描述
	Status      string   `json:"status"`      // 状态
	DataScope   string   `json:"data_scope"`  // 数据权限范围
	DeptIds     []string `json:"dept_ids"`    // 自定义部门权限ID列表
	IsBuiltin   bool     `json:"is_builtin"`  // 是否为系统内置角色
	SortOrder   int      `json:"sort_order"`  // 排序顺序
	Remark      *string  `json:"remark"`      // 角色备注
	UserCount   int      `json:"user_count"`  // 拥有此角色的用户数量
	MenuCount   int      `json:"menu_count"`  // 分配的菜单数量
	CreatedAt   string   `json:"created_at"`  // 创建时间
	UpdatedAt   string   `json:"updated_at"`  // 更新时间
}

// RoleOutput 角色输出
type RoleOutput struct {
	*RoleInfo
}

// ListRolesOutput 获取角色列表输出
type ListRolesOutput struct {
	Roles []*RoleInfo `json:"roles"` // 角色列表
	PageOutput
}

// AssignRoleMenusInput 分配角色菜单权限输入
type AssignRoleMenusInput struct {
	MenuIds []string `json:"menu_ids"` // 菜单ID列表
}

// Validate 验证分配角色菜单权限输入
func (i *AssignRoleMenusInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *AssignRoleMenusInput) Shape() z.Shape {
	return z.Shape{
		"MenuIds": z.Slice(z.String()).Optional(),
	}
}

// AssignRoleUsersInput 分配角色用户输入
type AssignRoleUsersInput struct {
	UserIds []string `json:"user_ids"` // 用户ID列表
}

// Validate 验证分配角色用户输入
func (i *AssignRoleUsersInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *AssignRoleUsersInput) Shape() z.Shape {
	return z.Shape{
		"UserIds": z.Slice(z.String()).Optional(),
	}
}

// RoleMenusOutput 角色菜单权限输出
type RoleMenusOutput struct {
	MenuIds []string `json:"menu_ids"` // 菜单ID列表
}

// RoleUsersOutput 角色用户输出
type RoleUsersOutput struct {
	Users []*UserInfo `json:"users"` // 用户列表
	PageOutput
}

// CheckRoleDeletableOutput 检查角色是否可删除输出
type CheckRoleDeletableOutput struct {
	Deletable bool   `json:"deletable"` // 是否可删除
	Reason    string `json:"reason"`    // 不可删除的原因
}

// BatchDeleteRolesInput 批量删除角色输入
type BatchDeleteRolesInput struct {
	RoleIds []string `json:"role_ids"` // 角色ID列表
}

// Validate 验证批量删除角色输入
func (i *BatchDeleteRolesInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *BatchDeleteRolesInput) Shape() z.Shape {
	return z.Shape{
		"RoleIds": z.Slice(z.String()).Min(1).Required(),
	}
}

// RoleStatsOutput 角色统计输出
type RoleStatsOutput struct {
	TotalRoles    int64            `json:"total_roles"`    // 总角色数
	EnabledRoles  int64            `json:"enabled_roles"`  // 启用角色数
	DisabledRoles int64            `json:"disabled_roles"` // 停用角色数
	BuiltinRoles  int64            `json:"builtin_roles"`  // 内置角色数
	StatusStats   map[string]int64 `json:"status_stats"`   // 状态统计
}
