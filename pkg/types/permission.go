package types

import (
	"time"

	z "github.com/Oudwins/zog"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// CreatePermissionInput 创建权限输入
type CreatePermissionInput struct {
	Name        string `json:"name"`        // 权限名称
	Code        string `json:"code"`        // 权限代码
	Resource    string `json:"resource"`    // 资源类型
	Action      string `json:"action"`      // 操作类型
	Description string `json:"description"` // 权限描述
	Status      string `json:"status"`      // 权限状态
	SortOrder   int    `json:"sort_order"`  // 排序顺序
}

// Validate 验证创建权限输入
func (i *CreatePermissionInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Name": z.String().
			Min(1, z.Message("权限名称不能为空")).
			Max(100, z.Message("权限名称长度不能超过100个字符")).
			Required(z.Message("权限名称是必填项")),
		"Code": z.String().
			Min(1, z.Message("权限代码不能为空")).
			Max(100, z.Message("权限代码长度不能超过100个字符")).
			Required(z.Message("权限代码是必填项")),
		"Resource": z.String().
			Min(1, z.Message("资源类型不能为空")).
			Max(50, z.Message("资源类型长度不能超过50个字符")).
			Required(z.Message("资源类型是必填项")),
		"Action": z.String().
			Min(1, z.Message("操作类型不能为空")).
			Max(50, z.Message("操作类型长度不能超过50个字符")).
			OneOf([]string{"create", "read", "update", "delete", "list", "import", "export"}, z.Message("操作类型不正确")).
			Required(z.Message("操作类型是必填项")),
		"Description": z.String().
			Max(255, z.Message("权限描述长度不能超过255个字符")).
			Optional(),
		"Status": z.String().
			OneOf([]string{"active", "inactive"}, z.Message("权限状态只能是active或inactive")).
			Default("active"),
		"SortOrder": z.Int().
			GTE(0, z.Message("排序顺序不能小于0")).
			Default(0),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// UpdatePermissionInput 更新权限输入
type UpdatePermissionInput struct {
	Name        *string `json:"name,omitempty"`        // 权限名称
	Description *string `json:"description,omitempty"` // 权限描述
	Status      *string `json:"status,omitempty"`      // 权限状态
	SortOrder   *int    `json:"sort_order,omitempty"`  // 排序顺序
}

// Validate 验证更新权限输入
func (i *UpdatePermissionInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Name": z.String().
			Min(1, z.Message("权限名称不能为空")).
			Max(100, z.Message("权限名称长度不能超过100个字符")).
			Optional(),
		"Description": z.String().
			Max(255, z.Message("权限描述长度不能超过255个字符")).
			Optional(),
		"Status": z.String().
			OneOf([]string{"active", "inactive"}, z.Message("权限状态只能是active或inactive")).
			Optional(),
		"SortOrder": z.Int().
			GTE(0, z.Message("排序顺序不能小于0")).
			Optional(),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// AssignPermissionInput 分配权限给角色输入
type AssignPermissionInput struct {
	RoleID        string   `json:"role_id"`        // 角色ID
	PermissionIDs []string `json:"permission_ids"` // 权限ID列表
}

// Validate 验证分配权限输入
func (i *AssignPermissionInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"RoleID": z.String().
			Len(26, z.Message("角色ID格式不正确")).
			Required(z.Message("角色ID是必填项")),
		"PermissionIDs": z.Slice(z.String().
			Len(26, z.Message("权限ID格式不正确"))).
			Min(1, z.Message("至少需要选择一个权限")).
			Required(z.Message("权限ID列表是必填项")),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// ListPermissionsInput 获取权限列表输入
type ListPermissionsInput struct {
	Page     int    `query:"page"`      // 页码
	PageSize int    `query:"page_size"` // 每页数量
	Resource string `query:"resource"`  // 资源类型过滤
	Action   string `query:"action"`    // 操作类型过滤
	Status   string `query:"status"`    // 状态过滤
	Search   string `query:"search"`    // 搜索关键词
}

// Validate 验证获取权限列表输入
func (i *ListPermissionsInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Page": z.Int().
			GTE(1, z.Message("页码不能小于1")).
			Default(1),
		"page_size": z.Int().
			GTE(1, z.Message("每页数量不能小于1")).
			LTE(100, z.Message("每页数量不能大于100")).
			Default(20),
		"Resource": z.String().
			Max(50, z.Message("资源类型长度不能超过50个字符")).
			Optional(),
		"Action": z.String().
			Max(50, z.Message("操作类型长度不能超过50个字符")).
			Optional(),
		"Status": z.String().
			OneOf([]string{"", "active", "inactive"}, z.Message("状态值无效")).
			Optional(),
		"Search": z.String().
			Max(100, z.Message("搜索关键词长度不能超过100个字符")).
			Optional(),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// PermissionOutput 权限输出
type PermissionOutput struct {
	ID          string    `json:"id"`          // 权限ID
	Name        string    `json:"name"`        // 权限名称
	Code        string    `json:"code"`        // 权限代码
	Resource    string    `json:"resource"`    // 资源类型
	Action      string    `json:"action"`      // 操作类型
	Description *string   `json:"description"` // 权限描述
	Status      string    `json:"status"`      // 权限状态
	IsSystem    bool      `json:"is_system"`   // 是否系统权限
	SortOrder   int       `json:"sort_order"`  // 排序顺序
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`  // 更新时间
}

// ListPermissionsOutput 权限列表输出
type ListPermissionsOutput struct {
	Permissions []*PermissionOutput `json:"permissions"` // 权限列表
	Total       int64               `json:"total"`       // 总数
	Page        int                 `json:"page"`        // 当前页码
	PageSize    int                 `json:"page_size"`   // 每页数量
	TotalPages  int                 `json:"total_pages"` // 总页数
}

// RolePermissionOutput 角色权限输出
type RolePermissionOutput struct {
	RoleID      string              `json:"role_id"`     // 角色ID
	RoleName    string              `json:"role_name"`   // 角色名称
	Permissions []*PermissionOutput `json:"permissions"` // 权限列表
}

// PermissionGroupOutput 权限分组输出（按资源分组）
type PermissionGroupOutput struct {
	Resource    string              `json:"resource"`    // 资源类型
	Permissions []*PermissionOutput `json:"permissions"` // 该资源下的权限列表
}
