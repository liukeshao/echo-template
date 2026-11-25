package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Token holds the schema definition for the Token entity.
type Token struct {
	ent.Schema
}

// Mixin 返回Token实体使用的mixin
func (Token) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the Token.
func (Token) Fields() []ent.Field {
	return []ent.Field{
		// 关联用户ID
		field.String("user_id").
			MaxLen(36).
			NotEmpty().
			Comment("关联的用户ID"),

		// Token值（JWT token）
		field.String("token").
			MaxLen(1000).
			NotEmpty().
			Sensitive().
			Comment("JWT token值"),

		// Token类型
		field.Enum("type").
			Values("access", "refresh").
			Comment("Token类型：access-访问令牌，refresh-刷新令牌"),

		// 过期时间
		field.Time("expires_at").
			Comment("Token过期时间"),

		// 是否已撤销
		field.Bool("is_revoked").
			Default(false).
			Comment("是否已撤销"),
	}
}

// Edges of the Token.
func (Token) Edges() []ent.Edge {
	return []ent.Edge{
		// 多个token属于一个用户
		edge.From("user", User.Type).
			Ref("tokens").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the Token.
func (Token) Indexes() []ent.Index {
	return []ent.Index{
		// Token值唯一索引（包含删除状态），覆盖 AuthenticateUser / RefreshToken 按 token 查询
		index.Fields("token", "deleted_at").
			Unique(),

		// 用户活跃令牌复合索引，覆盖 Logout / ChangePassword 按 user_id + is_revoked + deleted_at 批量撤销
		index.Fields("user_id", "is_revoked", "deleted_at"),

		// 过期时间索引（用于清理过期token）
		index.Fields("expires_at"),
	}
}
