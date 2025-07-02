package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/liukeshao/echo-template/pkg/types"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Mixin 返回User实体使用的mixin
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		// 用户名
		field.String("username").
			MaxLen(50).
			NotEmpty().
			Comment("用户名"),

		// 邮箱
		field.String("email").
			MaxLen(255).
			NotEmpty().
			Comment("用户邮箱"),

		// 密码哈希
		field.String("password_hash").
			MaxLen(255).
			NotEmpty().
			Sensitive().
			Comment("密码哈希值"),

		// 用户状态
		field.Enum("status").
			Values(types.UserStatuses()...).
			Default("active").
			Comment("用户状态：active-活跃，inactive-非活跃，suspended-停用"),

		// 最后登录时间
		field.Time("last_login_at").
			Optional().
			Nillable().
			Comment("最后登录时间"),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个用户可以有多个token
		edge.To("tokens", Token.Type),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		// 邮箱唯一索引（包含删除状态）
		index.Fields("email", "deleted_at").
			Unique(),

		// 用户名唯一索引（包含删除状态）
		index.Fields("username", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("status"),
		index.Fields("email"),
		index.Fields("username"),
	}
}
