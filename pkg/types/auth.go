package types

import (
	"time"

	z "github.com/Oudwins/zog"
	"github.com/golang-jwt/jwt/v5"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// RegisterInput 用户注册输入
type RegisterInput struct {
	Username string `json:"username" form:"username"`
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

// Validate 验证注册输入
func (i *RegisterInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		// 字段名必须与结构体字段名匹配，而不是输入数据的键名
		"Username": z.String().Min(3, z.Message("用户名长度不能小于3")).Max(50, z.Message("用户名长度不能大于50")).Required(z.Message("用户名不能为空")),
		"Email":    z.String().Email(z.Message("邮箱格式不正确")).Required(z.Message("邮箱不能为空")),
		"Password": z.String().Min(8, z.Message("密码长度不能小于8")).Required(z.Message("密码不能为空")),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// LoginInput 用户登录输入
type LoginInput struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

// Validate 验证登录输入
func (i *LoginInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"Email":    z.String().Email(z.Message("邮箱格式不正确")).Required(z.Message("邮箱不能为空")),
		"Password": z.String().Required(z.Message("密码不能为空")),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// RefreshTokenInput 刷新令牌输入
type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
}

// Validate 验证刷新令牌输入
func (i *RefreshTokenInput) Validate() []*errors.ErrorDetail {
	issuesMap := z.Struct(z.Shape{
		"RefreshToken": z.String().Required(z.Message("刷新令牌不能为空")),
	}).Validate(i)

	return ConvertZogIssues(issuesMap)
}

// AuthOutput 认证输出
type AuthOutput struct {
	User         *UserInfo `json:"user"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    int64     `json:"expires_at"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// JWTClaims JWT声明结构
type JWTClaims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	TokenType string `json:"token_type"` // "access" 或 "refresh"
	jwt.RegisteredClaims
}
