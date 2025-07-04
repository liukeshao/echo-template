package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/liukeshao/echo-template/pkg/types"
)

// Role holds the schema definition for the Role entity.
type Role struct {
	ent.Schema
}

// Mixin 返回Role实体使用的mixin
func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the Role.
func (Role) Fields() []ent.Field {
	return []ent.Field{
		// 角色名称
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("角色名称"),

		// 角色编码（唯一）
		field.String("code").
			MaxLen(50).
			NotEmpty().
			Comment("角色编码，全局唯一"),

		// 角色描述
		field.String("description").
			MaxLen(500).
			Optional().
			Comment("角色描述"),

		// 角色状态
		field.Enum("status").
			Values(types.RoleStatuses()...).
			Default("enabled").
			Comment("角色状态：enabled-启用，disabled-停用"),

		// 数据权限范围
		field.Enum("data_scope").
			Values(types.DataScopes()...).
			Default("all").
			Comment("数据权限范围：all-全部数据权限，dept_and_sub-本部门及以下数据权限，dept_only-本部门数据权限，self_only-本人数据权限"),

		// 自定义部门权限（当数据权限为自定义时使用）
		field.JSON("dept_ids", []string{}).
			Optional().
			Comment("自定义部门权限ID列表，当data_scope为custom时使用"),

		// 是否为系统内置角色（不可删除）
		field.Bool("is_builtin").
			Default(false).
			Comment("是否为系统内置角色，内置角色不可删除"),

		// 排序顺序
		field.Int("sort_order").
			Default(0).
			Comment("排序顺序，数字越小越靠前"),

		// 角色备注
		field.String("remark").
			MaxLen(500).
			Optional().
			Comment("角色备注"),
	}
}

// Edges of the Role.
func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		// 角色菜单关联表（一对多）
		edge.To("role_menus", RoleMenu.Type),

		// 用户角色关联表（一对多）
		edge.To("user_roles", UserRole.Type),
	}
}

// Indexes of the Role.
func (Role) Indexes() []ent.Index {
	return []ent.Index{
		// 角色名称唯一索引（包含删除状态）
		index.Fields("name", "deleted_at").
			Unique(),

		// 角色编码唯一索引（包含删除状态）
		index.Fields("code", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("status"),
		index.Fields("is_builtin"),
		index.Fields("sort_order"),
		index.Fields("data_scope"),

		// 复合索引优化查询
		index.Fields("status", "deleted_at"),
		index.Fields("status", "sort_order"),
		index.Fields("is_builtin", "status"),
	}
}
