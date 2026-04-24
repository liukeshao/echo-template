package schema

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"

	gen "github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/hook"
	"github.com/liukeshao/echo-template/ent/intercept"
)

// BaseMixin 基础字段混合器，包含所有实体的公共字段
type BaseMixin struct {
	mixin.Schema
}

// Fields 返回基础字段
func (BaseMixin) Fields() []ent.Field {
	return []ent.Field{
		// 主键：使用ULID
		field.String("id").
			MaxLen(26).
			MinLen(26).
			NotEmpty().
			Unique().
			Immutable().
			Comment("唯一标识符，ULID格式"),
	}
}

// TimeMixin 时间字段混合器，包含创建时间和更新时间
type TimeMixin struct {
	mixin.Schema
}

// Fields 返回时间相关字段
func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
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
	}
}

// SoftDeleteMixin 逻辑删除混合器 - 实现高级逻辑删除模式
type SoftDeleteMixin struct {
	mixin.Schema
}

// Fields 返回逻辑删除字段
func (SoftDeleteMixin) Fields() []ent.Field {
	return []ent.Field{
		// 逻辑删除时间戳（毫秒）
		field.Int64("deleted_at").
			Default(0).
			Comment("逻辑删除时间戳（毫秒），0表示未删除"),
	}
}

// Indexes 返回逻辑删除相关的索引
func (SoftDeleteMixin) Indexes() []ent.Index {
	return []ent.Index{
		// 逻辑删除状态索引，用于快速过滤已删除记录
		index.Fields("deleted_at"),
	}
}

type softDeleteKey struct{}

// SkipSoftDelete returns a new context that skips the soft-delete interceptor/mutators.
func SkipSoftDelete(parent context.Context) context.Context {
	return context.WithValue(parent, softDeleteKey{}, true)
}

// Interceptors of the SoftDeleteMixin.
func (d SoftDeleteMixin) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		intercept.TraverseFunc(func(ctx context.Context, q intercept.Query) error {
			// Skip soft-delete, means include soft-deleted entities.
			if skip, _ := ctx.Value(softDeleteKey{}).(bool); skip {
				return nil
			}
			d.P(q)
			return nil
		}),
	}
}

// Hooks 钩子 - 将删除操作转换为更新操作
func (d SoftDeleteMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(
			func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
					// Skip soft-delete, means delete the entity permanently.
					if skip, _ := ctx.Value(softDeleteKey{}).(bool); skip {
						return next.Mutate(ctx, m)
					}
					mx, ok := m.(interface {
						SetOp(ent.Op)
						Client() *gen.Client
						SetDeleteTime(int64)
						WhereP(...func(*sql.Selector))
					})
					if !ok {
						return nil, fmt.Errorf("unexpected mutation type %T", m)
					}
					d.P(mx)
					mx.SetOp(ent.OpUpdate)
					mx.SetDeleteTime(time.Now().UnixMilli())
					return mx.Client().Mutate(ctx, m)
				})
			},
			ent.OpDeleteOne|ent.OpDelete,
		),
	}
}

// P 添加存储级别的谓词到查询和变更中
func (d SoftDeleteMixin) P(w interface{ WhereP(...func(*sql.Selector)) }) {
	w.WhereP(
		sql.FieldEQ(d.Fields()[0].Descriptor().Name, 0),
	)
}

// DefaultMixin 默认混合器，组合了所有常用的mixin
type DefaultMixin struct {
	mixin.Schema
}

// Fields 组合所有默认字段
func (DefaultMixin) Fields() []ent.Field {
	fields := make([]ent.Field, 0)
	fields = append(fields, BaseMixin{}.Fields()...)
	fields = append(fields, TimeMixin{}.Fields()...)
	fields = append(fields, SoftDeleteMixin{}.Fields()...)
	return fields
}

// Indexes 组合所有默认索引
func (DefaultMixin) Indexes() []ent.Index {
	indexes := make([]ent.Index, 0)
	indexes = append(indexes, SoftDeleteMixin{}.Indexes()...)
	// 添加时间相关的索引
	indexes = append(indexes, index.Fields("created_at"))
	indexes = append(indexes, index.Fields("updated_at"))
	return indexes
}

// Interceptors 组合所有默认拦截器
func (DefaultMixin) Interceptors() []ent.Interceptor {
	return SoftDeleteMixin{}.Interceptors()
}

// Hooks 组合所有默认钩子
func (DefaultMixin) Hooks() []ent.Hook {
	return SoftDeleteMixin{}.Hooks()
}
