package utils

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

const (
	// 默认bcrypt成本值
	DefaultBcryptCost = 12
)

// bcryptCost 在生产环境使用 DefaultBcryptCost（12），
// 在 go test 环境下自动降为 bcrypt.MinCost（4）以加速测试执行。
func bcryptCost() int {
	if testing.Testing() {
		return bcrypt.MinCost
	}
	return DefaultBcryptCost
}

// HashPassword 对密码进行哈希加密
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost())
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// VerifyPassword 验证密码是否匹配哈希值
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
