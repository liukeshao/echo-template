package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Permission holds the schema definition for the Permission entity.
type Permission struct {
	ent.Schema
}

// Mixin 返回Permission实体使用的mixin
func (Permission) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the Permission.
func (Permission) Fields() []ent.Field {
	return []ent.Field{
		// 权限名称
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("权限名称"),

		// 权限代码（用于程序中标识）
		field.String("code").
			MaxLen(100).
			NotEmpty().
			Comment("权限代码，用于程序中标识，格式：resource:action"),

		// 资源类型
		field.String("resource").
			MaxLen(50).
			NotEmpty().
			Comment("资源类型，如：user, role, menu等"),

		// 操作类型
		field.String("action").
			MaxLen(50).
			NotEmpty().
			Comment("操作类型，如：create, read, update, delete"),

		// 权限描述
		field.String("description").
			MaxLen(255).
			Optional().
			Nillable().
			Comment("权限描述"),

		// 权限状态
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("权限状态：active-启用，inactive-禁用"),

		// 是否为系统内置权限
		field.Bool("is_system").
			Default(false).
			Comment("是否为系统内置权限，系统权限不可删除"),

		// 排序字段
		field.Int("sort_order").
			Default(0).
			Comment("排序顺序，数字越小越靠前"),
	}
}

// Edges of the Permission.
func (Permission) Edges() []ent.Edge {
	return []ent.Edge{
		// 拥有此权限的角色（多对多）
		edge.From("roles", Role.Type).
			Ref("permissions"),
	}
}

// Indexes of the Permission.
func (Permission) Indexes() []ent.Index {
	return []ent.Index{
		// 权限代码唯一索引（包含删除状态）
		index.Fields("code", "deleted_at").
			Unique(),

		// 资源和操作组合唯一索引（包含删除状态）
		index.Fields("resource", "action", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("resource"),
		index.Fields("action"),
		index.Fields("status"),
		index.Fields("is_system"),
		index.Fields("sort_order"),
	}
}
