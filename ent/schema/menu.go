package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/liukeshao/echo-template/pkg/types"
)

// Menu holds the schema definition for the Menu entity.
type Menu struct {
	ent.Schema
}

// Mixin 返回Menu实体使用的mixin
func (Menu) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the Menu.
func (Menu) Fields() []ent.Field {
	return []ent.Field{
		// 菜单名称
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("菜单名称"),

		// 菜单类型：目录(directory)、菜单(menu)、按钮(button)
		field.Enum("type").
			Values(types.MenuTypes()...).
			Comment("菜单类型：directory-目录，menu-菜单，button-按钮"),

		// 上级菜单ID（自关联）
		field.String("parent_id").
			MaxLen(26).
			Optional().
			Nillable().
			Comment("上级菜单ID，为空表示顶级菜单"),

		// 路由地址
		field.String("path").
			MaxLen(255).
			Optional().
			Comment("路由地址，菜单类型时必填，目录和按钮可为空"),

		// 组件路径
		field.String("component").
			MaxLen(255).
			Optional().
			Comment("组件路径，菜单类型时必填"),

		// 图标
		field.String("icon").
			MaxLen(100).
			Optional().
			Comment("菜单图标"),

		// 排序号
		field.Int("sort_order").
			Default(0).
			Comment("排序号，数字越小越靠前"),

		// 权限标识
		field.String("permission").
			MaxLen(255).
			Optional().
			Comment("权限标识，按钮类型时必填，格式如：system:user:add"),

		// 菜单状态
		field.Enum("status").
			Values(types.MenuStatuses()...).
			Default("enabled").
			Comment("菜单状态：enabled-启用，disabled-禁用"),

		// 是否显示
		field.Bool("visible").
			Default(true).
			Comment("是否在菜单中显示"),

		// 是否缓存
		field.Bool("keep_alive").
			Default(false).
			Comment("是否缓存页面（仅对菜单类型有效）"),

		// 外部链接
		field.String("external_link").
			MaxLen(500).
			Optional().
			Comment("外部链接地址，如果设置则点击菜单跳转到外部链接"),

		// 菜单备注
		field.String("remark").
			MaxLen(500).
			Optional().
			Comment("菜单备注"),
	}
}

// Edges of the Menu.
func (Menu) Edges() []ent.Edge {
	return []ent.Edge{
		// 上级菜单（多对一）
		edge.To("parent", Menu.Type).
			Unique().
			Field("parent_id"),

		// 子菜单（一对多）
		edge.From("children", Menu.Type).
			Ref("parent"),
	}
}

// Indexes of the Menu.
func (Menu) Indexes() []ent.Index {
	return []ent.Index{
		// 路由地址唯一索引（包含删除状态）
		index.Fields("path", "deleted_at").
			Unique(),

		// 权限标识唯一索引（包含删除状态）
		index.Fields("permission", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("type"),
		index.Fields("parent_id"),
		index.Fields("status"),
		index.Fields("visible"),
		index.Fields("sort_order"),

		// 复合索引优化查询
		index.Fields("parent_id", "sort_order"),
		index.Fields("type", "status"),
		index.Fields("parent_id", "status"),
		index.Fields("status", "visible"),
		index.Fields("status", "deleted_at"),
		index.Fields("parent_id", "status", "visible"),
	}
}
