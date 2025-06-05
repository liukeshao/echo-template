package schema

import (
	"time"

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

		// 授权者ID
		field.String("granted_by").
			MaxLen(26).
			Optional().
			Nillable().
			Comment("授权者ID，记录是谁给用户分配的角色"),

		// 授权时间
		field.Time("granted_at").
			Default(time.Now).
			Comment("授权时间"),

		// 过期时间
		field.Time("expires_at").
			Optional().
			Nillable().
			Comment("过期时间，为空表示永不过期"),

		// 状态
		field.Enum("status").
			Values("active", "inactive", "expired").
			Default("active").
			Comment("状态：active-生效，inactive-暂停，expired-过期"),

		// 备注
		field.String("remark").
			MaxLen(255).
			Optional().
			Nillable().
			Comment("备注信息"),
	}
}

// Edges of the UserRole.
func (UserRole) Edges() []ent.Edge {
	return []ent.Edge{
		// 关联到用户
		edge.To("user", User.Type).
			Field("user_id").
			Required().
			Unique(),

		// 关联到角色
		edge.To("role", Role.Type).
			Field("role_id").
			Required().
			Unique(),

		// 关联到授权者（用户）
		edge.To("granter", User.Type).
			Field("granted_by").
			Unique(),
	}
}

// Indexes of the UserRole.
func (UserRole) Indexes() []ent.Index {
	return []ent.Index{
		// 用户角色组合唯一索引（包含删除状态）
		index.Fields("user_id", "role_id", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("user_id"),
		index.Fields("role_id"),
		index.Fields("granted_by"),
		index.Fields("status"),
		index.Fields("expires_at"),
		index.Fields("granted_at"),
	}
}
