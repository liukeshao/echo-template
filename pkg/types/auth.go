package types

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/liukeshao/echo-template/pkg/validator"
)

// RegisterRequest 用户注册输入
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *RegisterRequest) Validate() error {
	var v validator.FieldViolations
	validator.CheckUsername(&v, "username", r.Username)
	if !validator.IsValidEmail(r.Email) {
		v.Add("email", validator.KeyEmail, map[string]string{"field": "email"})
	}
	validator.CheckPassword(&v, "password", r.Password)
	return v.Err()
}

// LoginRequest 用户登录输入
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate() error {
	var v validator.FieldViolations
	if !validator.IsValidEmail(r.Email) {
		v.Add("email", validator.KeyEmail, map[string]string{"field": "email"})
	}
	if strings.TrimSpace(r.Password) == "" {
		v.Add("password", validator.KeyRequired, map[string]string{"field": "password"})
	}
	return v.Err()
}

// RefreshTokenRequest 刷新令牌输入
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (r *RefreshTokenRequest) Validate() error {
	var v validator.FieldViolations
	if strings.TrimSpace(r.RefreshToken) == "" {
		v.Add("refresh_token", validator.KeyRequired, map[string]string{"field": "refresh_token"})
	}
	return v.Err()
}

// AuthResponse 认证输出
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// JWTClaims JWT声明结构
type JWTClaims struct {
	UserID    string `json:"user_id"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}
