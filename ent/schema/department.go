package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/liukeshao/echo-template/pkg/types"
)

// Department holds the schema definition for the Department entity.
type Department struct {
	ent.Schema
}

// Mixin 返回Department实体使用的mixin
func (Department) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the Department.
func (Department) Fields() []ent.Field {
	return []ent.Field{
		// 上级部门ID（树形结构的父节点）
		field.String("parent_id").
			MaxLen(26).
			Optional().
			Nillable().
			Comment("上级部门ID，根节点为null"),

		// 部门名称
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("部门名称"),

		// 部门编码（唯一）
		field.String("code").
			MaxLen(50).
			NotEmpty().
			Comment("部门编码，全局唯一"),

		// 负责人
		field.String("manager").
			MaxLen(100).
			Optional().
			Comment("部门负责人"),

		// 负责人ID
		field.String("manager_id").
			MaxLen(26).
			Optional().
			Comment("负责人用户ID"),

		// 联系电话
		field.String("phone").
			MaxLen(20).
			Optional().
			Comment("部门联系电话"),

		// 部门描述
		field.String("description").
			MaxLen(500).
			Optional().
			Comment("部门描述"),

		// 排序顺序
		field.Int("sort_order").
			Default(0).
			Comment("排序顺序，数字越小越靠前"),

		// 部门状态
		field.Enum("status").
			Values(types.DepartmentStatuses()...).
			Default("active").
			Comment("部门状态：active-启用，inactive-停用"),

		// 树形结构深度级别
		field.Int("level").
			Default(0).
			Comment("部门层级深度，根节点为0"),

		// 全路径（用于快速查找上级部门链）
		field.String("path").
			MaxLen(1000).
			Default("").
			Comment("部门全路径，格式：/root/parent/current"),
	}
}

// Edges of the Department.
func (Department) Edges() []ent.Edge {
	return []ent.Edge{
		// 自引用：上级部门（多对一）
		edge.To("parent", Department.Type).
			Unique().
			Field("parent_id"),

		// 自引用：下级部门（一对多）
		edge.From("children", Department.Type).
			Ref("parent"),

		// 部门下的用户（一对多）
		edge.From("users", User.Type).
			Ref("department_rel"),
	}
}

// Indexes of the Department.
func (Department) Indexes() []ent.Index {
	return []ent.Index{
		// 部门编码唯一索引（包含删除状态）
		index.Fields("code", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("parent_id"),
		index.Fields("status"),
		index.Fields("manager_id"),
		index.Fields("level"),
		index.Fields("sort_order"),

		// 复合索引优化查询
		index.Fields("parent_id", "status"),
		index.Fields("parent_id", "sort_order"),
		index.Fields("status", "deleted_at"),
		index.Fields("level", "sort_order"),

		// 路径索引（用于快速查找上级部门链）
		index.Fields("path"),
	}
}
