package schema

import (
	"context"
	"fmt"
	"slices"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"

	gen "github.com/liukeshao/go-rest-template/ent"
	"github.com/liukeshao/go-rest-template/ent/intercept"
)

type BaseMixin struct {
	mixin.Schema
}

func (BaseMixin) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			MaxLen(36).
			MinLen(36).
			NotEmpty().
			Unique().
			Immutable().
			Comment("唯一标识符，UUID v7 格式"),
	}
}

type TimeMixin struct {
	mixin.Schema
}

func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),
	}
}

const softDeleteField = "deleted_at"

// SoftDeleteMixin 为查询、更新与删除操作自动注入 deleted_at 逻辑删除语义。
type SoftDeleteMixin struct {
	mixin.Schema
}

func (SoftDeleteMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Int64(softDeleteField).
			Default(0).
			Comment("逻辑删除时间戳（毫秒），0表示未删除"),
	}
}

func (SoftDeleteMixin) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields(softDeleteField),
	}
}

type softDeleteKey struct{}

// SkipSoftDelete 返回跳过逻辑删除过滤的 context，用于查询或物理删除已软删记录。
func SkipSoftDelete(parent context.Context) context.Context {
	return context.WithValue(parent, softDeleteKey{}, true)
}

func (d SoftDeleteMixin) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		intercept.TraverseFunc(func(ctx context.Context, q intercept.Query) error {
			if skip, _ := ctx.Value(softDeleteKey{}).(bool); skip {
				return nil
			}
			d.P(q)
			return nil
		}),
	}
}

func (d SoftDeleteMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
				if skip, _ := ctx.Value(softDeleteKey{}).(bool); skip {
					return next.Mutate(ctx, m)
				}
				switch {
				case m.Op().Is(ent.OpDeleteOne | ent.OpDelete):
					mx, ok := m.(interface {
						SetOp(ent.Op)
						Client() *gen.Client
						SetDeletedAt(int64)
						WhereP(...func(*sql.Selector))
					})
					if !ok {
						return nil, fmt.Errorf("unexpected mutation type %T", m)
					}
					mx.SetOp(ent.OpUpdate)
					mx.SetDeletedAt(time.Now().UnixMilli())
					return mx.Client().Mutate(ctx, m)

				case m.Op().Is(ent.OpUpdate | ent.OpUpdateOne):
					if mx, ok := m.(interface{ WhereP(...func(*sql.Selector)) }); ok {
						d.P(mx)
					}
				}
				return next.Mutate(ctx, m)
			})
		},
	}
}

func (SoftDeleteMixin) P(w interface{ WhereP(...func(*sql.Selector)) }) {
	w.WhereP(sql.FieldEQ(softDeleteField, 0))
}

// DefaultMixin 组合 UUID v7 主键、时间戳与逻辑删除；时间列索引由各实体按需声明。
type DefaultMixin struct {
	mixin.Schema
}

func (DefaultMixin) Fields() []ent.Field {
	return slices.Concat(
		BaseMixin{}.Fields(),
		TimeMixin{}.Fields(),
		SoftDeleteMixin{}.Fields(),
	)
}

func (DefaultMixin) Indexes() []ent.Index {
	return SoftDeleteMixin{}.Indexes()
}

func (DefaultMixin) Interceptors() []ent.Interceptor {
	return SoftDeleteMixin{}.Interceptors()
}

func (DefaultMixin) Hooks() []ent.Hook {
	return SoftDeleteMixin{}.Hooks()
}
