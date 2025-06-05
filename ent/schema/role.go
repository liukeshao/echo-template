package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Role holds the schema definition for the Role entity.
type Role struct {
	ent.Schema
}

// Mixin 返回Role实体使用的mixin
func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the Role.
func (Role) Fields() []ent.Field {
	return []ent.Field{
		// 角色名称
		field.String("name").
			MaxLen(50).
			NotEmpty().
			Comment("角色名称"),

		// 角色代码（用于程序中标识）
		field.String("code").
			MaxLen(50).
			NotEmpty().
			Comment("角色代码，用于程序中标识"),

		// 角色描述
		field.String("description").
			MaxLen(255).
			Optional().
			Nillable().
			Comment("角色描述"),

		// 角色状态
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("角色状态：active-启用，inactive-禁用"),

		// 是否为系统内置角色
		field.Bool("is_system").
			Default(false).
			Comment("是否为系统内置角色，系统角色不可删除"),

		// 排序字段
		field.Int("sort_order").
			Default(0).
			Comment("排序顺序，数字越小越靠前"),
	}
}

// Edges of the Role.
func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		// 角色拥有的权限（多对多）
		edge.To("permissions", Permission.Type),

		// 拥有此角色的用户（多对多，通过UserRole表）
		edge.To("users", User.Type),
	}
}

// Indexes of the Role.
func (Role) Indexes() []ent.Index {
	return []ent.Index{
		// 角色代码唯一索引（包含删除状态）
		index.Fields("code", "deleted_at").
			Unique(),

		// 角色名称唯一索引（包含删除状态）
		index.Fields("name", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("status"),
		index.Fields("is_system"),
		index.Fields("sort_order"),
	}
}
