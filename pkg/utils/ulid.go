package utils

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// GenerateULID 生成新的ULID
func GenerateULID() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

// IsValidULID 验证ULID格式是否正确
func IsValidULID(str string) bool {
	_, err := ulid.Parse(str)
	return err == nil
}
