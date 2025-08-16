package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/samber/oops"
)

// EchoErrorHandler Echo框架的自定义错误处理器
func EchoErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		switch e := err.(type) {
		case oops.OopsError:
			handleOopsError(e, c, logger)
		case *echo.HTTPError:
			handleEchoHTTPError(e, c, logger)
		default:
			handleUnknownError(err, c, logger)
		}
	}
}

// handleOopsError 处理 oops 错误
func handleOopsError(err oops.OopsError, c echo.Context, logger *slog.Logger) {
	code := errors.InternalServerError
	if errCode := err.Code(); errCode != "" {
		if parsedCode, parseErr := strconv.Atoi(errCode); parseErr == nil {
			code = parsedCode
		}
	}

	message := oops.GetPublic(err, "服务暂时不可用")
	response := errors.NewErrorResponse(c, code, message)

	logError(logger, err, c, "Business error occurred")
	sendResponse(c, response, logger)
}

// handleEchoHTTPError 处理Echo HTTP错误
func handleEchoHTTPError(err *echo.HTTPError, c echo.Context, logger *slog.Logger) {
	logError(logger, err, c, "Echo HTTP error occurred")

	message := err.Message
	if message == nil {
		message = http.StatusText(err.Code)
	}

	c.JSON(err.Code, echo.Map{"message": message})
}

// handleUnknownError 处理未知错误
func handleUnknownError(err error, c echo.Context, logger *slog.Logger) {
	response := errors.NewErrorResponse(c, errors.InternalServerError, "内部服务器错误")

	logError(logger, err, c, "Unknown error occurred")
	sendResponse(c, response, logger)
}

// sendResponse 发送错误响应
func sendResponse(c echo.Context, response *errors.Response, logger *slog.Logger) {
	if err := c.JSON(http.StatusOK, response); err != nil {
		logger.Error("Failed to send error response",
			"error", err,
			"request_id", response.RequestID,
			"path", c.Request().URL.Path,
			"method", c.Request().Method,
		)
	}
}

// logError 统一的错误日志记录
func logError(logger *slog.Logger, err error, c echo.Context, msg string) {
	attrs := []any{
		"error", err,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"user_agent", c.Request().UserAgent(),
		"remote_addr", c.RealIP(),
	}

	// 根据错误类型选择日志级别
	switch err.(type) {
	case *echo.HTTPError:
		logger.WarnContext(c.Request().Context(), msg, attrs...)
	default:
		logger.ErrorContext(c.Request().Context(), msg, attrs...)
	}
}
