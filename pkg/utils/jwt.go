package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// JWT签名密钥（生产环境应从配置文件读取）
	DefaultJWTSecret = "your-super-secret-jwt-key-change-this-in-production"

	// Token过期时间
	AccessTokenExpiry  = 24 * time.Hour     // Access token 24小时过期
	RefreshTokenExpiry = 7 * 24 * time.Hour // Refresh token 7天过期
)

// JWTClaims JWT声明结构
type JWTClaims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	TokenType string `json:"token_type"` // "access" 或 "refresh"
	jwt.RegisteredClaims
}

// GenerateAccessToken 生成访问令牌
func GenerateAccessToken(userID, username, email string) (string, time.Time, error) {
	expirationTime := time.Now().Add(AccessTokenExpiry)

	claims := &JWTClaims{
		UserID:    userID,
		Username:  username,
		Email:     email,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "echo-template",
			Subject:   userID,
			ID:        GenerateULID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(DefaultJWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

// GenerateRefreshToken 生成刷新令牌
func GenerateRefreshToken(userID, username, email string) (string, time.Time, error) {
	expirationTime := time.Now().Add(RefreshTokenExpiry)

	claims := &JWTClaims{
		UserID:    userID,
		Username:  username,
		Email:     email,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "echo-template",
			Subject:   userID,
			ID:        GenerateULID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(DefaultJWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

// ValidateToken 验证JWT token
func ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 检查签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("无效的签名方法")
		}
		return []byte(DefaultJWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("无效的token")
	}

	return claims, nil
}

// IsTokenExpired 检查token是否过期
func IsTokenExpired(claims *JWTClaims) bool {
	return time.Now().After(claims.ExpiresAt.Time)
}

// GetTokenType 获取token类型
func GetTokenType(claims *JWTClaims) string {
	return claims.TokenType
}
