package service

import "golang.org/x/crypto/bcrypt"

const (
	// DefaultBcryptCost 是生产环境的默认 bcrypt 成本值。
	DefaultBcryptCost = 12

	// MinBcryptCost 是测试环境常用的最小 bcrypt 成本值。
	MinBcryptCost = bcrypt.MinCost
)

// resolveBcryptCost 将配置中的 bcrypt cost 规范化；未配置（<= 0）时回退到 DefaultBcryptCost。
func resolveBcryptCost(cost int) int {
	if cost <= 0 {
		return DefaultBcryptCost
	}
	return cost
}

// hashPassword 对密码进行哈希加密（集中管控 bcrypt cost 策略）。
func hashPassword(password string, cost int) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), resolveBcryptCost(cost))
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// verifyPassword 验证密码是否匹配哈希值。
func verifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
