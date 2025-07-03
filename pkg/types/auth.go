package types

import (
	"time"

	z "github.com/Oudwins/zog"
	"github.com/golang-jwt/jwt/v5"
)

// Token 类型常量
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// TokenTypes 返回所有 Token 类型值
func TokenTypes() []string {
	return []string{TokenTypeAccess, TokenTypeRefresh}
}

// RegisterInput 用户注册输入
type RegisterInput struct {
	Username string `json:"username" ` // 用户名
	Email    string `json:"email"`     // 邮箱
	Password string `json:"password"`  // 密码
}

// Validate 验证注册输入
func (i *RegisterInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	return FormatIssues(issuesMap)
}

func (i *RegisterInput) Shape() z.Shape {
	return z.Shape{
		"Username": z.String().Min(3).Max(50).Required(),
		"Email":    z.String().Email().Required(),
		"Password": z.String().Min(8).Required(),
	}
}

// LoginInput 用户登录输入
type LoginInput struct {
	Email    string `json:"email"`    // 邮箱
	Password string `json:"password"` // 密码
}

// Validate 验证登录输入
func (i *LoginInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *LoginInput) Shape() z.Shape {
	return z.Shape{
		"Email":    z.String().Email().Required(),
		"Password": z.String().Required(),
	}
}

// RefreshTokenInput 刷新令牌输入
type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token"` // 刷新令牌
}

// Validate 验证刷新令牌输入
func (i *RefreshTokenInput) Validate() []string {
	issuesMap := z.Struct(i.Shape()).Validate(i)

	return FormatIssues(issuesMap)
}

func (i *RefreshTokenInput) Shape() z.Shape {
	return z.Shape{
		"RefreshToken": z.String().Required(),
	}
}

// AuthOutput 认证输出
type AuthOutput struct {
	User         *UserInfo `json:"user"`          // 用户信息
	AccessToken  string    `json:"access_token"`  // 访问令牌
	RefreshToken string    `json:"refresh_token"` // 刷新令牌
	ExpiresAt    int64     `json:"expires_at"`    // 过期时间戳
}

// UserInfo 用户信息
type UserInfo struct {
	ID                  string     `json:"id"`                    // 用户ID
	Username            string     `json:"username"`              // 用户名
	Email               string     `json:"email"`                 // 邮箱
	RealName            *string    `json:"real_name,omitempty"`   // 真实姓名
	Phone               *string    `json:"phone,omitempty"`       // 手机号
	Department          *string    `json:"department,omitempty"`  // 所属部门
	Position            *string    `json:"position,omitempty"`    // 岗位
	Roles               []string   `json:"roles"`                 // 用户角色列表
	Status              string     `json:"status"`                // 状态
	ForceChangePassword bool       `json:"force_change_password"` // 是否强制修改密码
	AllowMultiLogin     bool       `json:"allow_multi_login"`     // 是否允许多端登录
	LastLoginAt         *time.Time `json:"last_login_at"`         // 最后登录时间
	LastLoginIP         *string    `json:"last_login_ip"`         // 最后登录IP
	CreatedAt           time.Time  `json:"created_at"`            // 创建时间
}

// JWTClaims JWT声明结构
type JWTClaims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	TokenType string `json:"token_type"` // TokenTypeAccess 或 TokenTypeRefresh
	jwt.RegisteredClaims
}
