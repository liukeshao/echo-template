package types

import (
	z "github.com/Oudwins/zog"
)

// 岗位状态常量
const (
	PositionStatusActive   = "active"   // 启用
	PositionStatusInactive = "inactive" // 停用
)

// PositionStatuses 返回所有岗位状态值
func PositionStatuses() []string {
	return []string{PositionStatusActive, PositionStatusInactive}
}

// CreatePositionInput 创建岗位输入
type CreatePositionInput struct {
	Name        string  `json:"name"`                  // 岗位名称
	Code        string  `json:"code"`                  // 岗位编码
	Description *string `json:"description,omitempty"` // 岗位描述
	SortOrder   int     `json:"sort_order,omitempty"`  // 排序顺序
	Status      string  `json:"status,omitempty"`      // 状态，默认为active
}

// Validate 验证创建岗位输入
func (i *CreatePositionInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *CreatePositionInput) Shape() z.Shape {
	return z.Shape{
		"Name":        z.String().Min(1).Max(100).Required(),
		"Code":        z.String().Min(1).Max(50).Required(),
		"Description": z.String().Max(500).Optional(),
		"SortOrder":   z.Int().Optional(),
		"Status":      z.String().OneOf(PositionStatuses()).Optional(),
	}
}

// UpdatePositionInput 更新岗位输入
type UpdatePositionInput struct {
	Name        *string `json:"name,omitempty"`        // 岗位名称
	Code        *string `json:"code,omitempty"`        // 岗位编码
	Description *string `json:"description,omitempty"` // 岗位描述
	SortOrder   *int    `json:"sort_order,omitempty"`  // 排序顺序
	Status      *string `json:"status,omitempty"`      // 状态
}

// Validate 验证更新岗位输入
func (i *UpdatePositionInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *UpdatePositionInput) Shape() z.Shape {
	return z.Shape{
		"Name":        z.String().Min(1).Max(100).Optional(),
		"Code":        z.String().Min(1).Max(50).Optional(),
		"Description": z.String().Max(500).Optional(),
		"SortOrder":   z.Int().Optional(),
		"Status":      z.String().OneOf(PositionStatuses()).Optional(),
	}
}

// SortPositionInput 岗位排序输入
type SortPositionInput struct {
	PositionSorts []PositionSortItem `json:"position_sorts"` // 岗位排序列表
}

// PositionSortItem 岗位排序项
type PositionSortItem struct {
	ID        string `json:"id"`         // 岗位ID
	SortOrder int    `json:"sort_order"` // 排序顺序
}

// Validate 验证岗位排序输入
func (i *SortPositionInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *SortPositionInput) Shape() z.Shape {
	return z.Shape{
		"PositionSorts": z.Slice(z.Struct(z.Shape{
			"ID":        z.String().Required(),
			"SortOrder": z.Int().Required(),
		})).Min(1).Required(),
	}
}

// ListPositionsInput 获取岗位列表输入
type ListPositionsInput struct {
	PageInput
	Status  string `query:"status"`  // 状态筛选
	Keyword string `query:"keyword"` // 用于搜索岗位名称、编码
}

// Validate 验证获取岗位列表输入
func (i *ListPositionsInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *ListPositionsInput) Shape() z.Shape {
	return z.Shape{
		"PageInput": z.Ptr(z.Struct(i.PageInput.Shape())),
		"Status":    z.String().OneOf(PositionStatuses()).Optional(),
		"Keyword":   z.String().Optional(),
	}
}

// PositionInfo 岗位信息
type PositionInfo struct {
	ID          string  `json:"id"`                    // 岗位ID
	Name        string  `json:"name"`                  // 岗位名称
	Code        string  `json:"code"`                  // 岗位编码
	Description *string `json:"description,omitempty"` // 岗位描述
	SortOrder   int     `json:"sort_order"`            // 排序顺序
	Status      string  `json:"status"`                // 状态
	CreatedAt   string  `json:"created_at"`            // 创建时间
	UpdatedAt   string  `json:"updated_at"`            // 更新时间
	UserCount   int64   `json:"user_count"`            // 岗位用户数量
}

// PositionOutput 岗位输出
type PositionOutput struct {
	*PositionInfo
}

// ListPositionsOutput 获取岗位列表输出
type ListPositionsOutput struct {
	Positions []*PositionInfo `json:"positions"` // 岗位列表
	PageOutput
}

// CheckPositionDeletableInput 检查岗位是否可删除输入
type CheckPositionDeletableInput struct {
	PositionID string `json:"position_id"` // 岗位ID
}

// Validate 验证检查岗位是否可删除输入
func (i *CheckPositionDeletableInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *CheckPositionDeletableInput) Shape() z.Shape {
	return z.Shape{
		"PositionID": z.String().Required(),
	}
}

// CheckPositionDeletableOutput 检查岗位是否可删除输出
type CheckPositionDeletableOutput struct {
	Deletable bool   `json:"deletable"`  // 是否可删除
	Reason    string `json:"reason"`     // 不可删除的原因
	UserCount int64  `json:"user_count"` // 关联用户数
}

// PositionStatsOutput 岗位统计输出
type PositionStatsOutput struct {
	TotalPositions    int64            `json:"total_positions"`    // 总岗位数
	ActivePositions   int64            `json:"active_positions"`   // 启用岗位数
	InactivePositions int64            `json:"inactive_positions"` // 停用岗位数
	StatusBreakdown   map[string]int64 `json:"status_breakdown"`   // 状态分布
}
