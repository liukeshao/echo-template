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

		var response *errors.Response

		switch e := err.(type) {
		case oops.OopsError:
			response = handleOopsError(e, c)
			logOopsError(logger, e, c)

		case *echo.HTTPError:
			handleEchoHTTPError(e, c, logger)
			return

		default:
			response = handleUnknownError(err, c)
			logUnknownError(logger, err, c)
		}

		sendErrorResponse(c, response, logger)
	}
}

// handleOopsError 处理 oops 错误
func handleOopsError(err oops.OopsError, c echo.Context) *errors.Response {
	code := errors.InternalServerError
	if errCode := err.Code(); errCode != "" {
		if parsedCode, parseErr := strconv.Atoi(errCode); parseErr == nil {
			code = parsedCode
		}
	}

	message := oops.GetPublic(err, "服务暂时不可用")
	return errors.NewErrorResponse(c, code, message)
}

// handleEchoHTTPError 处理Echo HTTP错误
func handleEchoHTTPError(err *echo.HTTPError, c echo.Context, logger *slog.Logger) {
	logEchoHTTPError(logger, err, c)

	message := err.Message
	if message == nil {
		message = http.StatusText(err.Code)
	}

	c.JSON(err.Code, echo.Map{"message": message})
}

// handleUnknownError 处理未知错误
func handleUnknownError(err error, c echo.Context) *errors.Response {
	return errors.NewErrorResponse(c, errors.InternalServerError, "内部服务器错误")
}

// sendErrorResponse 发送错误响应
func sendErrorResponse(c echo.Context, response *errors.Response, logger *slog.Logger) {
	if err := c.JSON(http.StatusOK, response); err != nil {
		logErrorResponseFailure(logger, err, c, response.RequestID)
	}
}

// logErrorResponseFailure 记录响应发送失败的日志
func logErrorResponseFailure(logger *slog.Logger, err error, c echo.Context, requestID string) {
	logger.Error("Failed to send error response",
		"error", err,
		"request_id", requestID,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
	)
}

// getRequestLogAttrs 获取请求的通用日志属性
func getRequestLogAttrs(c echo.Context) []any {
	return []any{
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"user_agent", c.Request().UserAgent(),
		"remote_addr", c.RealIP(),
	}
}

// logOopsError 记录 oops 错误日志
func logOopsError(logger *slog.Logger, err oops.OopsError, c echo.Context) {
	attrs := append([]any{"error", err}, getRequestLogAttrs(c)...)
	logger.ErrorContext(c.Request().Context(), "Business error occurred", attrs...)
}

// logEchoHTTPError 记录Echo HTTP错误日志
func logEchoHTTPError(logger *slog.Logger, err *echo.HTTPError, c echo.Context) {
	attrs := append([]any{"http_status", err.Code, "message", err.Message}, getRequestLogAttrs(c)...)
	logger.WarnContext(c.Request().Context(), "Echo HTTP error occurred", attrs...)
}

// logUnknownError 记录未知错误日志
func logUnknownError(logger *slog.Logger, err error, c echo.Context) {
	attrs := append([]any{"error", err.Error()}, getRequestLogAttrs(c)...)
	logger.ErrorContext(c.Request().Context(), "Unknown error occurred", attrs...)
}
