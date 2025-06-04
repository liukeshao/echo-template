package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Todo holds the schema definition for the Todo entity.
type Todo struct {
	ent.Schema
}

// Mixin 返回Todo实体使用的mixin
func (Todo) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the Todo.
func (Todo) Fields() []ent.Field {
	return []ent.Field{
		// 待办事项标题
		field.String("title").
			MaxLen(255).
			NotEmpty().
			Comment("待办事项标题"),

		// 待办事项描述
		field.Text("description").
			Optional().
			Comment("待办事项描述"),

		// 完成状态
		field.Bool("completed").
			Default(false).
			Comment("是否已完成"),

		// 优先级
		field.Enum("priority").
			Values("low", "medium", "high", "urgent").
			Default("medium").
			Comment("优先级：low-低，medium-中，high-高，urgent-紧急"),

		// 截止时间
		field.Time("due_date").
			Optional().
			Nillable().
			Comment("截止时间"),
	}
}

// Edges of the Todo.
func (Todo) Edges() []ent.Edge {
	return nil
}

// Indexes of the Todo.
func (Todo) Indexes() []ent.Index {
	return []ent.Index{
		// 标题索引
		index.Fields("title"),

		// 完成状态索引
		index.Fields("completed"),

		// 优先级索引
		index.Fields("priority"),

		// 截止时间索引
		index.Fields("due_date"),

		// 复合索引（完成状态 + 删除状态）
		index.Fields("completed", "deleted_at"),

		// 复合索引（优先级 + 删除状态）
		index.Fields("priority", "deleted_at"),
	}
}
