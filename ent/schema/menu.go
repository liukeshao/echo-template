package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Menu 菜单实体
type Menu struct {
	ent.Schema
}

// Mixin 应用默认混合器
func (Menu) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields 定义菜单字段
func (Menu) Fields() []ent.Field {
	return []ent.Field{
		// 菜单名称
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("菜单名称"),

		// 菜单标题（显示名称）
		field.String("title").
			MaxLen(100).
			NotEmpty().
			Comment("菜单标题"),

		// 菜单图标
		field.String("icon").
			MaxLen(100).
			Optional().
			Comment("菜单图标"),

		// 菜单路径/链接
		field.String("path").
			MaxLen(255).
			Optional().
			Comment("菜单路径"),

		// 菜单组件名称
		field.String("component").
			MaxLen(255).
			Optional().
			Comment("菜单组件"),

		// 父菜单ID
		field.String("parent_id").
			MaxLen(26).
			Optional().
			Nillable().
			Comment("父菜单ID"),

		// 菜单类型：menu-菜单，button-按钮，link-外链
		field.Enum("type").
			Values("menu", "button", "link").
			Default("menu").
			Comment("菜单类型"),

		// 菜单状态：active-启用，inactive-禁用
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("菜单状态"),

		// 是否隐藏
		field.Bool("hidden").
			Default(false).
			Comment("是否隐藏"),

		// 排序号
		field.Int("sort_order").
			Default(0).
			Comment("排序号"),

		// 权限标识
		field.String("permission").
			MaxLen(255).
			Optional().
			Comment("权限标识"),

		// 菜单描述
		field.String("description").
			MaxLen(500).
			Optional().
			Comment("菜单描述"),

		// 外链地址
		field.String("external_link").
			MaxLen(500).
			Optional().
			Comment("外链地址"),

		// 是否缓存
		field.Bool("keep_alive").
			Default(false).
			Comment("是否缓存"),

		// 面包屑中隐藏
		field.Bool("hide_breadcrumb").
			Default(false).
			Comment("面包屑中隐藏"),

		// 是否总是显示
		field.Bool("always_show").
			Default(false).
			Comment("是否总是显示"),
	}
}

// Edges 定义菜单关联关系
func (Menu) Edges() []ent.Edge {
	return []ent.Edge{
		// 父菜单
		edge.To("parent", Menu.Type).
			Unique().
			Field("parent_id"),

		// 子菜单
		edge.From("children", Menu.Type).
			Ref("parent"),

		// 拥有此菜单的角色（多对多）
		edge.From("roles", Role.Type).
			Ref("menus"),
	}
}

// Indexes 定义菜单索引
func (Menu) Indexes() []ent.Index {
	return []ent.Index{
		// 父菜单ID索引
		index.Fields("parent_id"),
		// 状态索引
		index.Fields("status"),
		// 类型索引
		index.Fields("type"),
		// 排序索引
		index.Fields("sort_order"),
		// 复合索引：父菜单ID + 排序
		index.Fields("parent_id", "sort_order"),
		// 复合索引：状态 + 排序
		index.Fields("status", "sort_order"),
		// 名称唯一索引（包含删除状态）
		index.Fields("name", "deleted_at").Unique(),
	}
}
