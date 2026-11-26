package rest

import (
	"net/http"
	"slices"
)

// StatusClientClosedRequest 是客户端提前断开连接时使用的非标准 HTTP 状态码（499）。
const StatusClientClosedRequest = 499

// Code 是有限的标准 HTTP 错误码枚举（对标 gRPC Canonical Codes）。
// 本身实现 error 接口，可直接作为不可变、零分配的哨兵错误与 errors.Is / oops.Wrap / oops.Join 配合使用。
type Code string

const (
	CodeBadRequest           Code = "BAD_REQUEST"            // 400
	CodeUnauthorized         Code = "UNAUTHORIZED"           // 401
	CodeForbidden            Code = "FORBIDDEN"              // 403
	CodeNotFound             Code = "NOT_FOUND"              // 404
	CodeMethodNotAllowed     Code = "METHOD_NOT_ALLOWED"     // 405
	CodeConflict             Code = "CONFLICT"               // 409
	CodeContentTooLarge      Code = "CONTENT_TOO_LARGE"      // 413
	CodeUnsupportedMediaType Code = "UNSUPPORTED_MEDIA_TYPE" // 415
	CodeTooManyRequests      Code = "TOO_MANY_REQUESTS"      // 429
	CodeClientClosedRequest  Code = "CLIENT_CLOSED_REQUEST"  // 499
	CodeInternal             Code = "INTERNAL_ERROR"         // 500
	CodeServiceUnavailable   Code = "SERVICE_UNAVAILABLE"    // 503
	CodeGatewayTimeout       Code = "GATEWAY_TIMEOUT"        // 504
)

var allCodes = []Code{
	CodeBadRequest,
	CodeUnauthorized,
	CodeForbidden,
	CodeNotFound,
	CodeMethodNotAllowed,
	CodeConflict,
	CodeContentTooLarge,
	CodeUnsupportedMediaType,
	CodeTooManyRequests,
	CodeClientClosedRequest,
	CodeInternal,
	CodeServiceUnavailable,
	CodeGatewayTimeout,
}

// AllCodes 返回全部有限标准 HTTP 错误码副本，供 i18n 启动覆盖率门禁检查使用。
func AllCodes() []Code {
	return slices.Clone(allCodes)
}

// Error 实现 error 接口，使每个 Code 常量本身即为标准哨兵错误。
func (c Code) Error() string { return string(c) }

// String 实现 fmt.Stringer。
func (c Code) String() string { return string(c) }

// HTTPStatus 返回标准错误码对应的唯一 HTTP 状态码；未知值回落 500 Internal Server Error。
func (c Code) HTTPStatus() int {
	switch c {
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case CodeConflict:
		return http.StatusConflict
	case CodeContentTooLarge:
		return http.StatusRequestEntityTooLarge
	case CodeUnsupportedMediaType:
		return http.StatusUnsupportedMediaType
	case CodeTooManyRequests:
		return http.StatusTooManyRequests
	case CodeClientClosedRequest:
		return StatusClientClosedRequest
	case CodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case CodeGatewayTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// CodeFromAny 从 oops.OopsError.Code() 等 any 值中提取标准 Code（兼容 Code 与 string）。
func CodeFromAny(v any) (Code, bool) {
	switch c := v.(type) {
	case Code:
		return c, c != ""
	case string:
		return Code(c), c != ""
	default:
		return "", false
	}
}
