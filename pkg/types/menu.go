package types

import (
	"time"

	z "github.com/Oudwins/zog"
)

// 菜单类型常量
const (
	MenuTypeDirectory = "directory" // 目录
	MenuTypeMenu      = "menu"      // 菜单
	MenuTypeButton    = "button"    // 按钮
)

// MenuTypes 返回所有菜单类型值
func MenuTypes() []string {
	return []string{MenuTypeDirectory, MenuTypeMenu, MenuTypeButton}
}

// 菜单状态常量
const (
	MenuStatusEnabled  = "enabled"  // 启用
	MenuStatusDisabled = "disabled" // 禁用
)

// MenuStatuses 返回所有菜单状态值
func MenuStatuses() []string {
	return []string{MenuStatusEnabled, MenuStatusDisabled}
}

// MenuInfo 菜单信息
type MenuInfo struct {
	ID           string      `json:"id"`                 // 菜单ID
	Name         string      `json:"name"`               // 菜单名称
	Type         string      `json:"type"`               // 菜单类型
	ParentID     *string     `json:"parent_id"`          // 上级菜单ID
	Path         *string     `json:"path"`               // 路由地址
	Component    *string     `json:"component"`          // 组件路径
	Icon         *string     `json:"icon"`               // 图标
	SortOrder    int         `json:"sort_order"`         // 排序号
	Permission   *string     `json:"permission"`         // 权限标识
	Status       string      `json:"status"`             // 状态
	Visible      bool        `json:"visible"`            // 是否显示
	KeepAlive    bool        `json:"keep_alive"`         // 是否缓存
	ExternalLink *string     `json:"external_link"`      // 外部链接
	Remark       *string     `json:"remark"`             // 备注
	CreatedAt    time.Time   `json:"created_at"`         // 创建时间
	UpdatedAt    time.Time   `json:"updated_at"`         // 更新时间
	Children     []*MenuInfo `json:"children,omitempty"` // 子菜单
}

// CreateMenuInput 创建菜单输入
type CreateMenuInput struct {
	Name         string  `json:"name"`                    // 菜单名称
	Type         string  `json:"type"`                    // 菜单类型
	ParentID     *string `json:"parent_id,omitempty"`     // 上级菜单ID
	Path         *string `json:"path,omitempty"`          // 路由地址
	Component    *string `json:"component,omitempty"`     // 组件路径
	Icon         *string `json:"icon,omitempty"`          // 图标
	SortOrder    int     `json:"sort_order,omitempty"`    // 排序号
	Permission   *string `json:"permission,omitempty"`    // 权限标识
	Status       string  `json:"status,omitempty"`        // 状态，默认为enabled
	Visible      bool    `json:"visible,omitempty"`       // 是否显示，默认为true
	KeepAlive    bool    `json:"keep_alive,omitempty"`    // 是否缓存，默认为false
	ExternalLink *string `json:"external_link,omitempty"` // 外部链接
	Remark       *string `json:"remark,omitempty"`        // 备注
}

// Validate 验证创建菜单输入
func (i *CreateMenuInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	errors := FormatIssues(issuesMap)

	// 自定义验证逻辑
	if i.Type == MenuTypeMenu {
		if i.Path == nil || *i.Path == "" {
			errors = append(errors, "菜单类型必须设置路由地址")
		}
		if i.Component == nil || *i.Component == "" {
			errors = append(errors, "菜单类型必须设置组件路径")
		}
	}

	if i.Type == MenuTypeButton {
		if i.Permission == nil || *i.Permission == "" {
			errors = append(errors, "按钮类型必须设置权限标识")
		}
	}

	return errors
}

func (i *CreateMenuInput) Shape() z.Shape {
	return z.Shape{
		"Name":         z.String().Min(1).Max(100).Required(),
		"Type":         z.String().OneOf(MenuTypes()).Required(),
		"ParentID":     z.String().Max(26).Optional(),
		"Path":         z.String().Max(255).Optional(),
		"Component":    z.String().Max(255).Optional(),
		"Icon":         z.String().Max(100).Optional(),
		"SortOrder":    z.Int().Optional(),
		"Permission":   z.String().Max(255).Optional(),
		"Status":       z.String().OneOf(MenuStatuses()).Optional(),
		"Visible":      z.Bool().Optional(),
		"KeepAlive":    z.Bool().Optional(),
		"ExternalLink": z.String().Max(500).Optional(),
		"Remark":       z.String().Max(500).Optional(),
	}
}

// UpdateMenuInput 更新菜单输入
type UpdateMenuInput struct {
	Name         *string `json:"name,omitempty"`          // 菜单名称
	Type         *string `json:"type,omitempty"`          // 菜单类型
	ParentID     *string `json:"parent_id,omitempty"`     // 上级菜单ID
	Path         *string `json:"path,omitempty"`          // 路由地址
	Component    *string `json:"component,omitempty"`     // 组件路径
	Icon         *string `json:"icon,omitempty"`          // 图标
	SortOrder    *int    `json:"sort_order,omitempty"`    // 排序号
	Permission   *string `json:"permission,omitempty"`    // 权限标识
	Status       *string `json:"status,omitempty"`        // 状态
	Visible      *bool   `json:"visible,omitempty"`       // 是否显示
	KeepAlive    *bool   `json:"keep_alive,omitempty"`    // 是否缓存
	ExternalLink *string `json:"external_link,omitempty"` // 外部链接
	Remark       *string `json:"remark,omitempty"`        // 备注
}

// Validate 验证更新菜单输入
func (i *UpdateMenuInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	errors := FormatIssues(issuesMap)

	// 自定义验证逻辑
	if i.Type != nil && *i.Type == MenuTypeMenu {
		if i.Path != nil && *i.Path == "" {
			errors = append(errors, "菜单类型不能设置空的路由地址")
		}
		if i.Component != nil && *i.Component == "" {
			errors = append(errors, "菜单类型不能设置空的组件路径")
		}
	}

	if i.Type != nil && *i.Type == MenuTypeButton {
		if i.Permission != nil && *i.Permission == "" {
			errors = append(errors, "按钮类型不能设置空的权限标识")
		}
	}

	return errors
}

func (i *UpdateMenuInput) Shape() z.Shape {
	return z.Shape{
		"Name":         z.String().Min(1).Max(100).Optional(),
		"Type":         z.String().OneOf(MenuTypes()).Optional(),
		"ParentID":     z.String().Max(26).Optional(),
		"Path":         z.String().Max(255).Optional(),
		"Component":    z.String().Max(255).Optional(),
		"Icon":         z.String().Max(100).Optional(),
		"SortOrder":    z.Int().Optional(),
		"Permission":   z.String().Max(255).Optional(),
		"Status":       z.String().OneOf(MenuStatuses()).Optional(),
		"Visible":      z.Bool().Optional(),
		"KeepAlive":    z.Bool().Optional(),
		"ExternalLink": z.String().Max(500).Optional(),
		"Remark":       z.String().Max(500).Optional(),
	}
}

// ListMenusInput 获取菜单列表输入
type ListMenusInput struct {
	Type     string `query:"type"`      // 菜单类型筛选
	ParentID string `query:"parent_id"` // 上级菜单ID筛选
	Status   string `query:"status"`    // 状态筛选
	Keyword  string `query:"keyword"`   // 关键词搜索（名称、权限标识）
}

// Validate 验证获取菜单列表输入
func (i *ListMenusInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *ListMenusInput) Shape() z.Shape {
	return z.Shape{
		"Type":     z.String().OneOf(MenuTypes()).Optional(),
		"ParentID": z.String().Max(26).Optional(),
		"Status":   z.String().OneOf(MenuStatuses()).Optional(),
		"Keyword":  z.String().Optional(),
	}
}

// MenuOutput 菜单输出
type MenuOutput struct {
	*MenuInfo
}

// ListMenusOutput 获取菜单列表输出
type ListMenusOutput struct {
	Menus []*MenuInfo `json:"menus"` // 菜单列表
}

// MenuTreeOutput 菜单树输出
type MenuTreeOutput struct {
	Tree []*MenuInfo `json:"tree"` // 菜单树
}

// SortMenuInput 菜单排序输入
type SortMenuInput struct {
	MenuItems []MenuSortItem `json:"menu_items"` // 菜单排序项
}

// MenuSortItem 菜单排序项
type MenuSortItem struct {
	ID        string `json:"id"`         // 菜单ID
	SortOrder int    `json:"sort_order"` // 新的排序号
}

// Validate 验证菜单排序输入
func (i *SortMenuInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *SortMenuInput) Shape() z.Shape {
	return z.Shape{
		"MenuItems": z.Slice(z.Struct(z.Shape{
			"ID":        z.String().Min(1).Required(),
			"SortOrder": z.Int().Required(),
		})).Min(1).Required(),
	}
}

// MoveMenuInput 移动菜单输入
type MoveMenuInput struct {
	ParentID *string `json:"parent_id"` // 新的上级菜单ID，null表示移动到根级
}

// Validate 验证移动菜单输入
func (i *MoveMenuInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *MoveMenuInput) Shape() z.Shape {
	return z.Shape{
		"ParentID": z.String().Max(26).Optional(),
	}
}

// CheckMenuDeletableOutput 检查菜单是否可删除输出
type CheckMenuDeletableOutput struct {
	Deletable bool     `json:"deletable"` // 是否可删除
	Reasons   []string `json:"reasons"`   // 不可删除的原因
}
