package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/liukeshao/echo-template/pkg/types"
)

// Position holds the schema definition for the Position entity.
type Position struct {
	ent.Schema
}

// Mixin 返回Position实体使用的mixin
func (Position) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the Position.
func (Position) Fields() []ent.Field {
	return []ent.Field{
		// 岗位名称
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("岗位名称"),

		// 岗位编码（唯一）
		field.String("code").
			MaxLen(50).
			NotEmpty().
			Comment("岗位编码，全局唯一"),

		// 岗位描述
		field.String("description").
			MaxLen(500).
			Optional().
			Comment("岗位描述"),

		// 排序顺序
		field.Int("sort_order").
			Default(0).
			Comment("排序顺序，数字越小越靠前"),

		// 岗位状态
		field.Enum("status").
			Values(types.PositionStatuses()...).
			Default("active").
			Comment("岗位状态：active-启用，inactive-停用"),
	}
}

// Edges of the Position.
func (Position) Edges() []ent.Edge {
	return []ent.Edge{
		// 岗位下的用户（一对多）
		edge.From("users", User.Type).
			Ref("position_rel"),
	}
}

// Indexes of the Position.
func (Position) Indexes() []ent.Index {
	return []ent.Index{
		// 岗位名称唯一索引（包含删除状态）
		index.Fields("name", "deleted_at").
			Unique(),

		// 岗位编码唯一索引（包含删除状态）
		index.Fields("code", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("status"),
		index.Fields("sort_order"),

		// 复合索引优化查询
		index.Fields("status", "deleted_at"),
		index.Fields("status", "sort_order"),
	}
}
