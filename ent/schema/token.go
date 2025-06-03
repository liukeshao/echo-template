package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Token holds the schema definition for the Token entity.
type Token struct {
	ent.Schema
}

// Fields of the Token.
func (Token) Fields() []ent.Field {
	return []ent.Field{
		// 主键：使用ULID
		field.String("id").
			MaxLen(26).
			MinLen(26).
			NotEmpty().
			Unique().
			Immutable().
			Comment("Token唯一标识符，ULID格式"),

		// 关联用户ID
		field.String("user_id").
			MaxLen(26).
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

		// 最后使用时间
		field.Time("last_used_at").
			Optional().
			Nillable().
			Comment("最后使用时间"),

		// 创建时间
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),

		// 更新时间
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),

		// 逻辑删除时间戳
		field.Int64("deleted_at").
			Default(0).
			Comment("逻辑删除时间戳，0表示未删除"),
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
		// Token值唯一索引（包含删除状态）
		index.Fields("token", "deleted_at").
			Unique(),

		// 用户ID索引
		index.Fields("user_id"),

		// 用户ID + 类型索引（用于查询特定用户的特定类型token）
		index.Fields("user_id", "type"),

		// 过期时间索引（用于清理过期token）
		index.Fields("expires_at"),

		// 查询优化索引
		index.Fields("deleted_at"),
		index.Fields("is_revoked"),
		index.Fields("type"),
		index.Fields("created_at"),

		// 复合索引（用户ID + 删除状态）
		index.Fields("user_id", "deleted_at"),
	}
}
