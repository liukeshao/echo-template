package utils

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	// 默认bcrypt成本值
	DefaultBcryptCost = 12
)

// HashPassword 对密码进行哈希加密
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultBcryptCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// VerifyPassword 验证密码是否匹配哈希值
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// IsPasswordValid 检查密码是否符合要求
func IsPasswordValid(password string) bool {
	// 密码长度至少8位
	return len(password) >= 8
}
