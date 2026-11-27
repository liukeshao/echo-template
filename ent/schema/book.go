package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Book struct {
	ent.Schema
}

func (Book) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

func (Book) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").
			MaxLen(200).
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
			Nillable().
			Comment("价格"),

		field.Time("published_at").
			Optional().
			Nillable().
			Comment("出版时间"),
	}
}

func (Book) Edges() []ent.Edge {
	return nil
}

func (Book) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("title"),
		index.Fields("author"),
		index.Fields("created_at"),
	}
}
