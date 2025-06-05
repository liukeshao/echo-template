package types

import (
	"time"

	z "github.com/Oudwins/zog"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// CreateMenuInput 创建菜单输入
type CreateMenuInput struct {
	Name           string  `json:"name"`
	Title          string  `json:"title"`
	Icon           *string `json:"icon,omitempty"`
	Path           *string `json:"path,omitempty"`
	Component      *string `json:"component,omitempty"`
	ParentID       *string `json:"parent_id,omitempty"`
	Type           string  `json:"type"`
	Status         string  `json:"status"`
	Hidden         bool    `json:"hidden"`
	SortOrder      int     `json:"sort_order"`
	Permission     *string `json:"permission,omitempty"`
	Description    *string `json:"description,omitempty"`
	ExternalLink   *string `json:"external_link,omitempty"`
	KeepAlive      bool    `json:"keep_alive"`
	HideBreadcrumb bool    `json:"hide_breadcrumb"`
	AlwaysShow     bool    `json:"always_show"`
}

// Validate 验证创建菜单输入
func (i *CreateMenuInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Name":           z.String().Min(1, z.Message("菜单名称不能为空")).Max(100, z.Message("菜单名称长度不能超过100")).Required(z.Message("菜单名称不能为空")),
		"Title":          z.String().Min(1, z.Message("菜单标题不能为空")).Max(100, z.Message("菜单标题长度不能超过100")).Required(z.Message("菜单标题不能为空")),
		"Icon":           z.String().Max(100, z.Message("图标长度不能超过100")).Optional(),
		"Path":           z.String().Max(255, z.Message("路径长度不能超过255")).Optional(),
		"Component":      z.String().Max(255, z.Message("组件名称长度不能超过255")).Optional(),
		"ParentID":       z.String().Len(26, z.Message("父菜单ID格式错误")).Optional(),
		"Type":           z.String().OneOf([]string{"menu", "button", "link"}, z.Message("菜单类型必须是 menu, button 或 link")).Default("menu"),
		"Status":         z.String().OneOf([]string{"active", "inactive"}, z.Message("菜单状态必须是 active 或 inactive")).Default("active"),
		"Hidden":         z.Bool().Default(false),
		"SortOrder":      z.Int().GTE(0, z.Message("排序号不能为负数")).Default(0),
		"Permission":     z.String().Max(255, z.Message("权限标识长度不能超过255")).Optional(),
		"Description":    z.String().Max(500, z.Message("描述长度不能超过500")).Optional(),
		"ExternalLink":   z.String().Max(500, z.Message("外链地址长度不能超过500")).Optional(),
		"KeepAlive":      z.Bool().Default(false),
		"HideBreadcrumb": z.Bool().Default(false),
		"AlwaysShow":     z.Bool().Default(false),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// UpdateMenuInput 更新菜单输入
type UpdateMenuInput struct {
	Name           *string `json:"name,omitempty"`
	Title          *string `json:"title,omitempty"`
	Icon           *string `json:"icon,omitempty"`
	Path           *string `json:"path,omitempty"`
	Component      *string `json:"component,omitempty"`
	ParentID       *string `json:"parent_id,omitempty"`
	Type           *string `json:"type,omitempty"`
	Status         *string `json:"status,omitempty"`
	Hidden         *bool   `json:"hidden,omitempty"`
	SortOrder      *int    `json:"sort_order,omitempty"`
	Permission     *string `json:"permission,omitempty"`
	Description    *string `json:"description,omitempty"`
	ExternalLink   *string `json:"external_link,omitempty"`
	KeepAlive      *bool   `json:"keep_alive,omitempty"`
	HideBreadcrumb *bool   `json:"hide_breadcrumb,omitempty"`
	AlwaysShow     *bool   `json:"always_show,omitempty"`
}

// Validate 验证更新菜单输入
func (i *UpdateMenuInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Name":           z.String().Min(1, z.Message("菜单名称不能为空")).Max(100, z.Message("菜单名称长度不能超过100")).Optional(),
		"Title":          z.String().Min(1, z.Message("菜单标题不能为空")).Max(100, z.Message("菜单标题长度不能超过100")).Optional(),
		"Icon":           z.String().Max(100, z.Message("图标长度不能超过100")).Optional(),
		"Path":           z.String().Max(255, z.Message("路径长度不能超过255")).Optional(),
		"Component":      z.String().Max(255, z.Message("组件名称长度不能超过255")).Optional(),
		"ParentID":       z.String().Len(26, z.Message("父菜单ID格式错误")).Optional(),
		"Type":           z.String().OneOf([]string{"menu", "button", "link"}, z.Message("菜单类型必须是 menu, button 或 link")).Optional(),
		"Status":         z.String().OneOf([]string{"active", "inactive"}, z.Message("菜单状态必须是 active 或 inactive")).Optional(),
		"Hidden":         z.Bool().Optional(),
		"SortOrder":      z.Int().GTE(0, z.Message("排序号不能为负数")).Optional(),
		"Permission":     z.String().Max(255, z.Message("权限标识长度不能超过255")).Optional(),
		"Description":    z.String().Max(500, z.Message("描述长度不能超过500")).Optional(),
		"ExternalLink":   z.String().Max(500, z.Message("外链地址长度不能超过500")).Optional(),
		"KeepAlive":      z.Bool().Optional(),
		"HideBreadcrumb": z.Bool().Optional(),
		"AlwaysShow":     z.Bool().Optional(),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// ListMenusInput 获取菜单列表输入
type ListMenusInput struct {
	Page     int     `query:"page"`
	PageSize int     `query:"page_size"`
	ParentID *string `query:"parent_id"`
	Type     *string `query:"type"`
	Status   *string `query:"status"`
	Keyword  *string `query:"keyword"`
	TreeMode bool    `query:"tree_mode"` // 是否返回树形结构
}

// Validate 验证获取菜单列表输入
func (i *ListMenusInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Page":     z.Int().GTE(1, z.Message("页码必须大于0")).Default(1),
		"PageSize": z.Int().GTE(1, z.Message("每页数量必须大于0")).LTE(100, z.Message("每页数量不能超过100")).Default(20),
		"ParentID": z.String().Len(26, z.Message("父菜单ID格式错误")).Optional(),
		"Type":     z.String().OneOf([]string{"menu", "button", "link"}, z.Message("菜单类型必须是 menu, button 或 link")).Optional(),
		"Status":   z.String().OneOf([]string{"active", "inactive"}, z.Message("菜单状态必须是 active 或 inactive")).Optional(),
		"Keyword":  z.String().Optional(),
		"TreeMode": z.Bool().Default(false),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// UpdateMenuOrderInput 更新菜单排序输入
type UpdateMenuOrderInput struct {
	MenuOrders []MenuOrderItem `json:"menu_orders"`
}

// MenuOrderItem 菜单排序项
type MenuOrderItem struct {
	ID        string  `json:"id"`
	ParentID  *string `json:"parent_id"`
	SortOrder int     `json:"sort_order"`
}

// Validate 验证更新菜单排序输入
func (i *UpdateMenuOrderInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"MenuOrders": z.Slice(z.Struct(z.Shape{
			"ID":        z.String().Len(26, z.Message("菜单ID格式错误")).Required(z.Message("菜单ID不能为空")),
			"ParentID":  z.String().Len(26, z.Message("父菜单ID格式错误")).Optional(),
			"SortOrder": z.Int().GTE(0, z.Message("排序号不能为负数")).Required(z.Message("排序号不能为空")),
		})).Min(1, z.Message("至少需要一个菜单项")).Required(z.Message("菜单排序数据不能为空")),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// MenuOutput 菜单输出
type MenuOutput struct {
	*MenuInfo
}

// MenuInfo 菜单信息
type MenuInfo struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Title          string      `json:"title"`
	Icon           *string     `json:"icon,omitempty"`
	Path           *string     `json:"path,omitempty"`
	Component      *string     `json:"component,omitempty"`
	ParentID       *string     `json:"parent_id,omitempty"`
	Type           string      `json:"type"`
	Status         string      `json:"status"`
	Hidden         bool        `json:"hidden"`
	SortOrder      int         `json:"sort_order"`
	Permission     *string     `json:"permission,omitempty"`
	Description    *string     `json:"description,omitempty"`
	ExternalLink   *string     `json:"external_link,omitempty"`
	KeepAlive      bool        `json:"keep_alive"`
	HideBreadcrumb bool        `json:"hide_breadcrumb"`
	AlwaysShow     bool        `json:"always_show"`
	Children       []*MenuInfo `json:"children,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// ListMenusOutput 获取菜单列表输出
type ListMenusOutput struct {
	Menus      []*MenuInfo `json:"menus"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// MenuTreeOutput 菜单树输出
type MenuTreeOutput struct {
	Menus []*MenuInfo `json:"menus"`
	Total int64       `json:"total"`
}

// MenuStatsOutput 菜单统计输出
type MenuStatsOutput struct {
	TotalMenus    int64            `json:"total_menus"`
	ActiveMenus   int64            `json:"active_menus"`
	InactiveMenus int64            `json:"inactive_menus"`
	MenusByType   map[string]int64 `json:"menus_by_type"`
}
