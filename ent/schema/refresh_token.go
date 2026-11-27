package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RefreshToken 仅持久化刷新令牌的 SHA-256 哈希；Access Token 为无状态 JWT 不落库。
type RefreshToken struct {
	ent.Schema
}

func (RefreshToken) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

func (RefreshToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").
			MaxLen(36).
			NotEmpty().
			Comment("关联的用户 ID"),

		field.String("family_id").
			MaxLen(36).
			NotEmpty().
			Comment("刷新令牌族 ID"),

		field.String("token_hash").
			MaxLen(64).
			NotEmpty().
			Sensitive().
			Comment("刷新令牌的 SHA-256 哈希值"),

		field.Time("expires_at").
			Comment("刷新令牌过期时间"),

		field.Bool("is_revoked").
			Default(false).
			Comment("是否已撤销"),
	}
}

func (RefreshToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("refresh_tokens").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (RefreshToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token_hash", "deleted_at").Unique(),
		index.Fields("user_id", "is_revoked", "deleted_at"),
		index.Fields("family_id", "is_revoked", "deleted_at"),
		index.Fields("expires_at"),
	}
}
