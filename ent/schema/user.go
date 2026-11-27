package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct {
	ent.Schema
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").
			MaxLen(50).
			NotEmpty().
			Comment("用户名"),

		field.String("email").
			MaxLen(255).
			NotEmpty().
			Comment("用户邮箱"),

		field.String("password_hash").
			MaxLen(255).
			NotEmpty().
			Sensitive().
			Comment("密码哈希值"),

		field.Enum("status").
			Values("active", "inactive", "suspended").
			Default("active").
			Comment("用户状态：active-活跃，inactive-非活跃，suspended-停用"),

		field.Time("last_login_at").
			Optional().
			Nillable().
			Comment("最后登录时间"),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("refresh_tokens", RefreshToken.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email", "deleted_at").Unique(),
		index.Fields("username", "deleted_at").Unique(),
		index.Fields("status"),
	}
}
