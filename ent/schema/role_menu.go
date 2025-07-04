package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RoleMenu holds the schema definition for the RoleMenu entity.
type RoleMenu struct {
	ent.Schema
}

// Mixin 返回RoleMenu实体使用的mixin
func (RoleMenu) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the RoleMenu.
func (RoleMenu) Fields() []ent.Field {
	return []ent.Field{
		// 角色ID
		field.String("role_id").
			MaxLen(26).
			NotEmpty().
			Comment("角色ID"),

		// 菜单ID
		field.String("menu_id").
			MaxLen(26).
			NotEmpty().
			Comment("菜单ID"),
	}
}

// Edges of the RoleMenu.
func (RoleMenu) Edges() []ent.Edge {
	return []ent.Edge{
		// 关联到角色
		edge.From("role", Role.Type).
			Ref("role_menus").
			Field("role_id").
			Required().
			Unique(),

		// 关联到菜单
		edge.From("menu", Menu.Type).
			Ref("role_menus").
			Field("menu_id").
			Required().
			Unique(),
	}
}

// Indexes of the RoleMenu.
func (RoleMenu) Indexes() []ent.Index {
	return []ent.Index{
		// 角色ID和菜单ID的复合唯一索引（包含删除状态）
		index.Fields("role_id", "menu_id", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("role_id"),
		index.Fields("menu_id"),

		// 复合索引优化查询
		index.Fields("role_id", "deleted_at"),
		index.Fields("menu_id", "deleted_at"),
	}
}
