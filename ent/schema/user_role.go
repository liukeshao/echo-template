package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserRole holds the schema definition for the UserRole entity.
type UserRole struct {
	ent.Schema
}

// Mixin 返回UserRole实体使用的mixin
func (UserRole) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the UserRole.
func (UserRole) Fields() []ent.Field {
	return []ent.Field{
		// 用户ID
		field.String("user_id").
			MaxLen(26).
			NotEmpty().
			Comment("用户ID"),

		// 角色ID
		field.String("role_id").
			MaxLen(26).
			NotEmpty().
			Comment("角色ID"),
	}
}

// Edges of the UserRole.
func (UserRole) Edges() []ent.Edge {
	return []ent.Edge{
		// 关联到用户
		edge.From("user", User.Type).
			Ref("user_roles").
			Field("user_id").
			Required().
			Unique(),

		// 关联到角色
		edge.From("role", Role.Type).
			Ref("user_roles").
			Field("role_id").
			Required().
			Unique(),
	}
}

// Indexes of the UserRole.
func (UserRole) Indexes() []ent.Index {
	return []ent.Index{
		// 用户ID和角色ID的复合唯一索引（包含删除状态）
		index.Fields("user_id", "role_id", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("user_id"),
		index.Fields("role_id"),

		// 复合索引优化查询
		index.Fields("user_id", "deleted_at"),
		index.Fields("role_id", "deleted_at"),
	}
}
