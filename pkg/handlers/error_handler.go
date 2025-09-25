package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/pkg/apperrs"
)

// AppErrorHandler 应用的自定义错误处理器
func AppErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	switch e := err.(type) {
	case oops.OopsError:
		handleOopsError(e, c)
	case *echo.HTTPError:
		handleEchoHTTPError(e, c)
	default:
		handleUnknownError(err, c)
	}
}

// handleOopsError 处理 oops 错误
func handleOopsError(err oops.OopsError, c echo.Context) {
	code := apperrs.InternalServerError
	if errCode := err.Code(); errCode != "" {
		if parsedCode, parseErr := strconv.Atoi(errCode); parseErr == nil {
			code = parsedCode
		}
	}

	message := oops.GetPublic(err, "服务暂时不可用")
	response := apperrs.NewResponse(c, apperrs.WithCode(code), apperrs.WithMessage(message))

	logError(err, c, "Business error occurred")
	sendResponse(c, response)
}

// handleEchoHTTPError 处理Echo HTTP错误
func handleEchoHTTPError(err *echo.HTTPError, c echo.Context) {
	logError(err, c, "Echo HTTP error occurred")

	message := err.Message
	if message == nil {
		message = http.StatusText(err.Code)
	}

	c.JSON(err.Code, echo.Map{"message": message})
}

// handleUnknownError 处理未知错误
func handleUnknownError(err error, c echo.Context) {
	response := apperrs.NewResponse(c, apperrs.WithCode(apperrs.InternalServerError), apperrs.WithMessage("内部服务器错误"))

	logError(err, c, "Unknown error occurred")
	sendResponse(c, response)
}

// sendResponse 发送错误响应
func sendResponse(c echo.Context, response *apperrs.Response) {
	if err := c.JSON(http.StatusOK, response); err != nil {
		logger := slog.With(
			"request_id", response.RequestID,
			"path", c.Request().URL.Path,
			"method", c.Request().Method,
		)
		logger.ErrorContext(c.Request().Context(), "Failed to send error response", "error", err)
	}
}

// logError 统一的错误日志记录
func logError(err error, c echo.Context, msg string) {
	logger := slog.With(
		"error", err,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"user_agent", c.Request().UserAgent(),
		"remote_addr", c.RealIP(),
	)

	// 根据错误类型选择日志级别
	switch err.(type) {
	case *echo.HTTPError:
		logger.WarnContext(c.Request().Context(), msg)
	default:
		logger.ErrorContext(c.Request().Context(), msg)
	}
}
