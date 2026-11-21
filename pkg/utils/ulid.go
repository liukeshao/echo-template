package utils

import (
	"github.com/oklog/ulid/v2"
)

// GenerateULID 生成新的ULID
func GenerateULID() string {
	return ulid.Make().String()
}

// IsValidULID 验证ULID格式是否正确
func IsValidULID(str string) bool {
	_, err := ulid.Parse(str)
	return err == nil
}
