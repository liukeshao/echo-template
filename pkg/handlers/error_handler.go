package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// EchoErrorHandler Echo框架的自定义错误处理器
// 统一处理所有的错误响应，HTTP状态码统一返回200
func EchoErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		// 如果响应已经被提交，则不能再修改响应
		if c.Response().Committed {
			return
		}

		// 获取请求ID用于日志记录和响应
		requestID := ""
		if id, ok := context.GetRequestIDFromContext(c.Request().Context()); ok {
			requestID = id
		}

		var response *Response

		// 根据错误类型处理
		switch e := err.(type) {
		case *errors.AppError:
			// 自定义业务错误
			response = handleAppError(e, requestID)
			logAppError(logger, e, c)

		case *echo.HTTPError:
			// Echo框架HTTP错误，使用Echo原生策略处理
			handleEchoHTTPError(e, c, logger)
			return

		default:
			// 未知错误，作为内部服务器错误处理
			response = handleUnknownError(err, requestID)
			logUnknownError(logger, err, c)
		}

		// 发送JSON响应，HTTP状态码统一为200
		if err := c.JSON(http.StatusOK, response); err != nil {
			// 如果发送响应失败，记录日志
			logger.Error("Failed to send error response",
				"error", err,
				"request_id", requestID,
				"path", c.Request().URL.Path,
				"method", c.Request().Method,
			)
		}
	}
}

// handleAppError 处理自定义业务错误
func handleAppError(err *errors.AppError, requestID string) *Response {
	response := &Response{
		Code:      err.Code(),
		Message:   err.Message(),
		Data:      nil,
		Errors:    []string{err.Error()},
		Timestamp: time.Now().Unix(),
		RequestID: requestID,
	}
	return response
}

// handleEchoHTTPError 使用Echo原生策略处理HTTP错误
func handleEchoHTTPError(err *echo.HTTPError, c echo.Context, logger *slog.Logger) {
	// 记录HTTP错误日志
	logEchoHTTPError(logger, err, c)

	// 使用Echo原生的HTTP错误响应格式
	// 如果有内部错误信息，只返回状态码对应的标准消息
	message := err.Message
	if message == nil {
		message = http.StatusText(err.Code)
	}

	// 直接返回HTTP状态码和消息，不使用我们的自定义响应格式
	c.JSON(err.Code, echo.Map{
		"message": message,
	})
}

// handleUnknownError 处理未知错误
func handleUnknownError(err error, requestID string) *Response {
	return &Response{
		Code:      errors.InternalServerError,
		Message:   "Internal Server Error",
		Data:      nil,
		Errors:    []string{err.Error()},
		Timestamp: time.Now().Unix(),
		RequestID: requestID,
	}
}

// 日志记录函数

// logAppError 记录业务错误日志
func logAppError(logger *slog.Logger, err *errors.AppError, c echo.Context) {
	logger.ErrorContext(c.Request().Context(),
		"Business error occurred",
		"error", err,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"user_agent", c.Request().UserAgent(),
		"remote_addr", c.RealIP(),
	)
}

// logEchoHTTPError 记录Echo HTTP错误日志
func logEchoHTTPError(logger *slog.Logger, err *echo.HTTPError, c echo.Context) {
	logger.WarnContext(c.Request().Context(),
		"Echo HTTP error occurred",
		"http_status", err.Code,
		"message", err.Message,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"user_agent", c.Request().UserAgent(),
		"remote_addr", c.RealIP(),
	)
}

// logUnknownError 记录未知错误日志
func logUnknownError(logger *slog.Logger, err error, c echo.Context) {
	logger.ErrorContext(c.Request().Context(),
		"Unknown error occurred",
		"error", err.Error(),
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"user_agent", c.Request().UserAgent(),
		"remote_addr", c.RealIP(),
	)
}
