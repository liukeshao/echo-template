package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/liukeshao/echo-template/pkg/types"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Mixin 返回User实体使用的mixin
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		DefaultMixin{},
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		// 用户名
		field.String("username").
			MaxLen(50).
			NotEmpty().
			Comment("用户名"),

		// 邮箱
		field.String("email").
			MaxLen(255).
			NotEmpty().
			Comment("用户邮箱"),

		// 密码哈希
		field.String("password_hash").
			MaxLen(255).
			NotEmpty().
			Sensitive().
			Comment("密码哈希值"),

		// 真实姓名
		field.String("real_name").
			MaxLen(100).
			Optional().
			Comment("真实姓名"),

		// 手机号
		field.String("phone").
			MaxLen(20).
			Optional().
			Comment("手机号"),

		// 所属部门
		field.String("department").
			MaxLen(100).
			Optional().
			Comment("所属部门"),

		// 所属部门ID（关联字段）
		field.String("department_id").
			MaxLen(26).
			Optional().
			Comment("所属部门ID"),

		// 岗位
		field.String("position").
			MaxLen(100).
			Optional().
			Comment("岗位"),

		// 岗位ID（关联字段）
		field.String("position_id").
			MaxLen(26).
			Optional().
			Comment("岗位ID"),

		// 用户状态
		field.Enum("status").
			Values(types.UserStatuses()...).
			Default("active").
			Comment("用户状态：active-活跃，inactive-非活跃，suspended-停用"),

		// 是否需要强制修改密码
		field.Bool("force_change_password").
			Default(false).
			Comment("是否强制修改密码"),

		// 是否允许多端登录
		field.Bool("allow_multi_login").
			Default(true).
			Comment("是否允许多端登录"),

		// 最后登录时间
		field.Time("last_login_at").
			Optional().
			Nillable().
			Comment("最后登录时间"),

		// 最后登录IP
		field.String("last_login_ip").
			MaxLen(45).
			Optional().
			Comment("最后登录IP"),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个用户可以有多个token
		edge.To("tokens", Token.Type),

		// 用户所属部门（多对一）
		edge.To("department_rel", Department.Type).
			Unique().
			Field("department_id"),

		// 用户所属岗位（多对一）
		edge.To("position_rel", Position.Type).
			Unique().
			Field("position_id"),

		// 用户角色关联表（一对多）
		edge.To("user_roles", UserRole.Type),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		// 邮箱唯一索引（包含删除状态）
		index.Fields("email", "deleted_at").
			Unique(),

		// 用户名唯一索引（包含删除状态）
		index.Fields("username", "deleted_at").
			Unique(),

		// 手机号唯一索引（包含删除状态）
		index.Fields("phone", "deleted_at").
			Unique(),

		// 查询优化索引
		index.Fields("status"),
		index.Fields("email"),
		index.Fields("username"),
		index.Fields("phone"),
		index.Fields("department"),
		index.Fields("department_id"),
		index.Fields("position"),
		index.Fields("position_id"),
		index.Fields("force_change_password"),
		index.Fields("allow_multi_login"),

		// 复合索引优化查询
		index.Fields("department", "status"),
		index.Fields("department_id", "status"),
		index.Fields("position", "status"),
		index.Fields("position_id", "status"),
		index.Fields("status", "deleted_at"),
	}
}
