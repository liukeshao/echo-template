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

	gen "github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/intercept"
)

// BaseMixin 基础字段混合器，定义所有实体通用的 UUID v7 主键。
type BaseMixin struct {
	mixin.Schema
}

// Fields 返回基础主键字段。
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

// TimeMixin 时间字段混合器，包含创建时间与更新时间。
type TimeMixin struct {
	mixin.Schema
}

// Fields 返回时间字段。
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

// SoftDeleteMixin 逻辑删除混合器，自动拦截查询、更新与删除操作。
type SoftDeleteMixin struct {
	mixin.Schema
}

// Fields 返回逻辑删除时间戳字段（毫秒，0 表示未删除）。
func (SoftDeleteMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Int64(softDeleteField).
			Default(0).
			Comment("逻辑删除时间戳（毫秒），0表示未删除"),
	}
}

// Indexes 返回逻辑删除状态索引。
func (SoftDeleteMixin) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields(softDeleteField),
	}
}

type softDeleteKey struct{}

// SkipSoftDelete 返回跳过逻辑删除拦截器与钩子的 context（用于查询或物理删除已软删记录）。
func SkipSoftDelete(parent context.Context) context.Context {
	return context.WithValue(parent, softDeleteKey{}, true)
}

// Interceptors 为所有查询自动附加 deleted_at = 0 过滤条件。
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

// Hooks 钩子：
//   - 将 Delete / DeleteOne 转换为设置 deleted_at 的逻辑删除更新。
//   - 为 Update / UpdateOne 自动附加 deleted_at = 0 条件，防止误修改已逻辑删除的记录。
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

// P 向查询或变更追加 deleted_at = 0 存储级谓词。
func (SoftDeleteMixin) P(w interface{ WhereP(...func(*sql.Selector)) }) {
	w.WhereP(sql.FieldEQ(softDeleteField, 0))
}

// DefaultMixin 默认混合器，组合 BaseMixin、TimeMixin 与 SoftDeleteMixin。
// 各表如需按 created_at / updated_at 排序或过滤，请在具体实体 Schema 中按需建立索引。
type DefaultMixin struct {
	mixin.Schema
}

// Fields 组合全部默认字段。
func (DefaultMixin) Fields() []ent.Field {
	return slices.Concat(
		BaseMixin{}.Fields(),
		TimeMixin{}.Fields(),
		SoftDeleteMixin{}.Fields(),
	)
}

// Indexes 组合默认逻辑删除索引。
func (DefaultMixin) Indexes() []ent.Index {
	return SoftDeleteMixin{}.Indexes()
}

// Interceptors 组合默认拦截器。
func (DefaultMixin) Interceptors() []ent.Interceptor {
	return SoftDeleteMixin{}.Interceptors()
}

// Hooks 组合默认钩子。
func (DefaultMixin) Hooks() []ent.Hook {
	return SoftDeleteMixin{}.Hooks()
}
