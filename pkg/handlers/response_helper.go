package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// Success 成功响应
func Success(c echo.Context, data any) error {
	response := NewResponse(c).
		WithCode(errors.OK).
		WithMessage("Success").
		WithData(data)
	return c.JSON(http.StatusOK, response)
}

// Error 错误响应
func Error(c echo.Context, code int, message string) error {
	response := NewResponse(c).
		WithCode(code).
		WithMessage(message).
		WithData(nil)
	return c.JSON(http.StatusOK, response)
}

// ValidationError 验证错误响应
func ValidationError(c echo.Context, errs []string) error {
	response := NewResponse(c).
		WithCode(errors.UnprocessableEntity).
		WithMessage("Parameter validation failed").
		WithErrors(errs)
	return c.JSON(http.StatusOK, response)
}
