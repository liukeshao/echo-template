package types

import (
	z "github.com/Oudwins/zog"
)

// 部门状态常量
const (
	DepartmentStatusActive   = "active"   // 启用
	DepartmentStatusInactive = "inactive" // 停用
)

// DepartmentStatuses 返回所有部门状态值
func DepartmentStatuses() []string {
	return []string{DepartmentStatusActive, DepartmentStatusInactive}
}

// CreateDepartmentInput 创建部门输入
type CreateDepartmentInput struct {
	ParentID    *string `json:"parent_id,omitempty"`   // 上级部门ID
	Name        string  `json:"name"`                  // 部门名称
	Code        string  `json:"code"`                  // 部门编码
	Manager     *string `json:"manager,omitempty"`     // 负责人
	ManagerID   *string `json:"manager_id,omitempty"`  // 负责人ID
	Phone       *string `json:"phone,omitempty"`       // 联系电话
	Description *string `json:"description,omitempty"` // 部门描述
	SortOrder   int     `json:"sort_order,omitempty"`  // 排序顺序
	Status      string  `json:"status,omitempty"`      // 状态，默认为active
}

// Validate 验证创建部门输入
func (i *CreateDepartmentInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *CreateDepartmentInput) Shape() z.Shape {
	return z.Shape{
		"ParentID":    z.String().Max(26).Optional(),
		"Name":        z.String().Min(1).Max(100).Required(),
		"Code":        z.String().Min(1).Max(50).Required(),
		"Manager":     z.String().Max(100).Optional(),
		"ManagerID":   z.String().Max(26).Optional(),
		"Phone":       z.String().Max(20).Optional(),
		"Description": z.String().Max(500).Optional(),
		"SortOrder":   z.Int().Optional(),
		"Status":      z.String().OneOf(DepartmentStatuses()).Optional(),
	}
}

// UpdateDepartmentInput 更新部门输入
type UpdateDepartmentInput struct {
	Name        *string `json:"name,omitempty"`        // 部门名称
	Code        *string `json:"code,omitempty"`        // 部门编码
	Manager     *string `json:"manager,omitempty"`     // 负责人
	ManagerID   *string `json:"manager_id,omitempty"`  // 负责人ID
	Phone       *string `json:"phone,omitempty"`       // 联系电话
	Description *string `json:"description,omitempty"` // 部门描述
	SortOrder   *int    `json:"sort_order,omitempty"`  // 排序顺序
	Status      *string `json:"status,omitempty"`      // 状态
}

// Validate 验证更新部门输入
func (i *UpdateDepartmentInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *UpdateDepartmentInput) Shape() z.Shape {
	return z.Shape{
		"Name":        z.String().Min(1).Max(100).Optional(),
		"Code":        z.String().Min(1).Max(50).Optional(),
		"Manager":     z.String().Max(100).Optional(),
		"ManagerID":   z.String().Max(26).Optional(),
		"Phone":       z.String().Max(20).Optional(),
		"Description": z.String().Max(500).Optional(),
		"SortOrder":   z.Int().Optional(),
		"Status":      z.String().OneOf(DepartmentStatuses()).Optional(),
	}
}

// MoveDepartmentInput 移动部门输入（调整父节点）
type MoveDepartmentInput struct {
	ParentID *string `json:"parent_id"` // 新的上级部门ID，null表示移动到根级
}

// Validate 验证移动部门输入
func (i *MoveDepartmentInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *MoveDepartmentInput) Shape() z.Shape {
	return z.Shape{
		"ParentID": z.String().Max(26).Optional(),
	}
}

// SortDepartmentInput 部门排序输入
type SortDepartmentInput struct {
	DepartmentSorts []DepartmentSortItem `json:"department_sorts"` // 部门排序列表
}

// DepartmentSortItem 部门排序项
type DepartmentSortItem struct {
	ID        string `json:"id"`         // 部门ID
	SortOrder int    `json:"sort_order"` // 排序顺序
}

// Validate 验证部门排序输入
func (i *SortDepartmentInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *SortDepartmentInput) Shape() z.Shape {
	return z.Shape{
		"DepartmentSorts": z.Slice(z.Struct(z.Shape{
			"ID":        z.String().Required(),
			"SortOrder": z.Int().Required(),
		})).Min(1).Required(),
	}
}

// ListDepartmentsInput 获取部门列表输入
type ListDepartmentsInput struct {
	PageInput
	ParentID  *string `query:"parent_id"`  // 上级部门ID，null获取根级部门
	Status    string  `query:"status"`     // 状态筛选
	Level     *int    `query:"level"`      // 层级筛选
	ManagerID *string `query:"manager_id"` // 负责人筛选
	Keyword   string  `query:"keyword"`    // 用于搜索部门名称、编码、负责人
}

// Validate 验证获取部门列表输入
func (i *ListDepartmentsInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *ListDepartmentsInput) Shape() z.Shape {
	return z.Shape{
		"PageInput": z.Ptr(z.Struct(i.PageInput.Shape())),
		"ParentID":  z.String().Max(26).Optional(),
		"Status":    z.String().OneOf(DepartmentStatuses()).Optional(),
		"Level":     z.Int().Optional(),
		"ManagerID": z.String().Max(26).Optional(),
		"Keyword":   z.String().Optional(),
	}
}

// DepartmentInfo 部门信息
type DepartmentInfo struct {
	ID          string            `json:"id"`                     // 部门ID
	ParentID    *string           `json:"parent_id"`              // 上级部门ID
	Name        string            `json:"name"`                   // 部门名称
	Code        string            `json:"code"`                   // 部门编码
	Manager     *string           `json:"manager,omitempty"`      // 负责人
	ManagerID   *string           `json:"manager_id,omitempty"`   // 负责人ID
	Phone       *string           `json:"phone,omitempty"`        // 联系电话
	Description *string           `json:"description,omitempty"`  // 部门描述
	SortOrder   int               `json:"sort_order"`             // 排序顺序
	Status      string            `json:"status"`                 // 状态
	Level       int               `json:"level"`                  // 层级深度
	Path        string            `json:"path"`                   // 全路径
	CreatedAt   string            `json:"created_at"`             // 创建时间
	UpdatedAt   string            `json:"updated_at"`             // 更新时间
	UserCount   int64             `json:"user_count"`             // 部门用户数量
	Children    []*DepartmentInfo `json:"children,omitempty"`     // 子部门列表
	Parent      *DepartmentInfo   `json:"parent,omitempty"`       // 上级部门信息
	ManagerInfo *UserInfo         `json:"manager_info,omitempty"` // 负责人信息
}

// DepartmentOutput 部门输出
type DepartmentOutput struct {
	*DepartmentInfo
}

// ListDepartmentsOutput 获取部门列表输出
type ListDepartmentsOutput struct {
	Departments []*DepartmentInfo `json:"departments"` // 部门列表
	PageOutput
}

// DepartmentTreeOutput 部门树形结构输出
type DepartmentTreeOutput struct {
	Departments []*DepartmentInfo `json:"departments"` // 部门树形列表
}

// CheckDepartmentDeletableInput 检查部门是否可删除输入
type CheckDepartmentDeletableInput struct {
	DepartmentID string `json:"department_id"` // 部门ID
}

// Validate 验证检查部门是否可删除输入
func (i *CheckDepartmentDeletableInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *CheckDepartmentDeletableInput) Shape() z.Shape {
	return z.Shape{
		"DepartmentID": z.String().Required(),
	}
}

// CheckDepartmentDeletableOutput 检查部门是否可删除输出
type CheckDepartmentDeletableOutput struct {
	Deletable     bool   `json:"deletable"`      // 是否可删除
	Reason        string `json:"reason"`         // 不可删除的原因
	UserCount     int64  `json:"user_count"`     // 关联用户数
	ChildrenCount int64  `json:"children_count"` // 子部门数
}

// DepartmentStatsOutput 部门统计输出
type DepartmentStatsOutput struct {
	TotalDepartments    int64            `json:"total_departments"`    // 总部门数
	ActiveDepartments   int64            `json:"active_departments"`   // 启用部门数
	InactiveDepartments int64            `json:"inactive_departments"` // 停用部门数
	StatusBreakdown     map[string]int64 `json:"status_breakdown"`     // 状态分布
	LevelStats          []LevelStat      `json:"level_stats"`          // 层级统计
}

// LevelStat 层级统计
type LevelStat struct {
	Level           int   `json:"level"`            // 层级
	DepartmentCount int64 `json:"department_count"` // 部门数量
}
