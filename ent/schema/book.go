package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Book holds the schema definition for the Book entity.
type Book struct {
	ent.Schema
}

// Mixin 返回Book实体使用的mixin
func (Book) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the Book.
func (Book) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").
			MaxLen(255).
			NotEmpty().
			Comment("书名"),

		field.String("author").
			MaxLen(100).
			NotEmpty().
			Comment("作者"),

		field.String("description").
			MaxLen(1000).
			Optional().
			Comment("简介"),

		field.Float("price").
			Optional().
			Comment("价格"),

		field.Time("published_at").
			Optional().
			Nillable().
			Comment("出版时间"),
	}
}

// Edges of the Book.
func (Book) Edges() []ent.Edge {
	return nil
}

// Indexes of the Book.
func (Book) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("title"),
		index.Fields("author"),
	}
}
