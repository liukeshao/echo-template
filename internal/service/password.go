package service

import "golang.org/x/crypto/bcrypt"

const (
	DefaultBcryptCost = 12
	MinBcryptCost     = bcrypt.MinCost
)

func resolveBcryptCost(cost int) int {
	if cost <= 0 {
		return DefaultBcryptCost
	}
	return cost
}

func hashPassword(password string, cost int) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), resolveBcryptCost(cost))
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

func verifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
