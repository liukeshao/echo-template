package errors

import (
	"database/sql"
	"errors"
	"fmt"
)

// Wrap 将普通error转换为AppError
func Wrap(err error) *AppError {
	if err == nil {
		return nil
	}

	// 如果已经是AppError，直接返回
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}

	// 检查特定错误类型并转换
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return NotFoundError("Record not found").Wrap(err)
	case errors.Is(err, sql.ErrConnDone):
		return DatabaseErrorf("Database connection closed").Wrap(err)
	default:
		return InternalError("Internal server error").Wrap(err)
	}
}

// Join 创建错误链，用于组合多个错误
func Join(errors ...error) *AppError {
	var nonNil []error
	for _, err := range errors {
		if err != nil {
			nonNil = append(nonNil, err)
		}
	}

	if len(nonNil) == 0 {
		return nil
	}

	if len(nonNil) == 1 {
		return Wrap(nonNil[0])
	}

	// 组合多个错误
	appErr := Wrap(nonNil[0])
	for i := 1; i < len(nonNil); i++ {
		appErr = appErr.With(fmt.Sprintf("additional_error_%d", i), nonNil[i].Error())
	}

	return appErr
}
