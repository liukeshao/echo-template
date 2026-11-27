package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RefreshToken 定义持久化的刷新令牌表（仅存 SHA-256 哈希，不存明文；Access Token 为无状态 JWT 不落库）。
type RefreshToken struct {
	ent.Schema
}

// Mixin 返回 RefreshToken 实体使用的公共 Mixin。
func (RefreshToken) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields 定义 RefreshToken 的数据列。
func (RefreshToken) Fields() []ent.Field {
	return []ent.Field{
		// 关联用户 ID
		field.String("user_id").
			MaxLen(36).
			NotEmpty().
			Comment("关联的用户 ID"),

		// 刷新令牌族 ID（同一次登录及其轮换链共享同一 family_id，用于重放检测与整链吊销）
		field.String("family_id").
			MaxLen(36).
			NotEmpty().
			Comment("刷新令牌族 ID"),

		// 刷新令牌的 SHA-256 十六进制哈希（固定 64 字符，不落明文）
		field.String("token_hash").
			MaxLen(64).
			NotEmpty().
			Sensitive().
			Comment("刷新令牌的 SHA-256 哈希值"),

		// 过期时间
		field.Time("expires_at").
			Comment("刷新令牌过期时间"),

		// 是否已撤销
		field.Bool("is_revoked").
			Default(false).
			Comment("是否已撤销"),
	}
}

// Edges 定义 RefreshToken 与 User 的关联。
func (RefreshToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("refresh_tokens").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes 定义 RefreshToken 的查询索引。
func (RefreshToken) Indexes() []ent.Index {
	return []ent.Index{
		// 令牌哈希唯一索引（定长 64 字符，兼容 MySQL utf8mb4），覆盖 RefreshToken 按哈希查找
		index.Fields("token_hash", "deleted_at").
			Unique(),

		// 用户活跃令牌复合索引，覆盖 Logout / ChangePassword 按 user_id 批量撤销
		index.Fields("user_id", "is_revoked", "deleted_at"),

		// 令牌族索引，覆盖重放检测时按 family_id 整链撤销
		index.Fields("family_id", "is_revoked", "deleted_at"),

		// 过期时间索引（供定期清理过期记录使用）
		index.Fields("expires_at"),
	}
}
