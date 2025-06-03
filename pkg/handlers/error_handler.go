package handlers

import (
	"log/slog"
	"net/http"

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

		var response *ResponseBuilder

		// 根据错误类型处理
		switch e := err.(type) {
		case *errors.AppError:
			// 自定义业务错误
			response = handleAppError(e, requestID)
			logAppError(logger, e, c)

		case *echo.HTTPError:
			// Echo框架HTTP错误
			response = handleHTTPError(e, requestID)
			logHTTPError(logger, e, c)

		default:
			// 未知错误，作为内部服务器错误处理
			response = handleUnknownError(err, requestID)
			logUnknownError(logger, err, c)
		}

		// 发送JSON响应，HTTP状态码统一为200
		if err := response.JSON(c); err != nil {
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
func handleAppError(err *errors.AppError, requestID string) *ResponseBuilder {
	response := Error(err.GetCode(), err.GetMessage()).WithRequestID(requestID)

	// 将context字段转换为错误详情
	if context := err.GetContext(); len(context) > 0 {
		var errorDetails []ErrorDetail
		for field, value := range context {
			errorDetails = append(errorDetails, ErrorDetail{
				Field:   field,
				Message: toString(value),
			})
		}
		response = response.WithErrors(errorDetails)
	}

	return response
}

// handleHTTPError 处理Echo HTTP错误
func handleHTTPError(err *echo.HTTPError, requestID string) *ResponseBuilder {
	code := err.Code
	message := "Unknown Error"

	// 获取错误消息
	if err.Message != nil {
		if msg, ok := err.Message.(string); ok {
			message = msg
		}
	}

	// 映射HTTP状态码到业务错误码
	businessCode := mapHTTPStatusToBusinessCode(code)

	return Error(businessCode, message).WithRequestID(requestID)
}

// handleUnknownError 处理未知错误
func handleUnknownError(err error, requestID string) *ResponseBuilder {
	return Error(errors.InternalServerError, "Internal Server Error").
		WithRequestID(requestID).
		WithError("system", err.Error(), "UNKNOWN_ERROR")
}

// mapHTTPStatusToBusinessCode 映射HTTP状态码到业务错误码
func mapHTTPStatusToBusinessCode(httpStatus int) int {
	switch httpStatus {
	case http.StatusBadRequest:
		return errors.BadRequest
	case http.StatusUnauthorized:
		return errors.Unauthorized
	case http.StatusForbidden:
		return errors.Forbidden
	case http.StatusNotFound:
		return errors.NotFound
	case http.StatusMethodNotAllowed:
		return errors.MethodNotAllowed
	case http.StatusConflict:
		return errors.Conflict
	case http.StatusUnprocessableEntity:
		return errors.UnprocessableEntity
	case http.StatusTooManyRequests:
		return errors.TooManyRequests
	case http.StatusInternalServerError:
		return errors.InternalServerError
	case http.StatusNotImplemented:
		return errors.NotImplemented
	case http.StatusBadGateway:
		return errors.BadGateway
	case http.StatusServiceUnavailable:
		return errors.ServiceUnavailable
	case http.StatusGatewayTimeout:
		return errors.GatewayTimeout
	default:
		return errors.InternalServerError
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

// logHTTPError 记录HTTP错误日志
func logHTTPError(logger *slog.Logger, err *echo.HTTPError, c echo.Context) {
	logger.WarnContext(c.Request().Context(),
		"HTTP error occurred",
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

// 工具函数

// toString 将interface{}转换为string
func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}
