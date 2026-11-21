// Package status 提供 google.rpc.Code 与 HTTP 状态码之间的双向映射。
// 参考: https://google.aip.dev/193#http11json-representation
package status

import (
	"net/http"

	"github.com/liukeshao/echo-template/transport/rpc/codes"
)

// codeToHTTP 将 google.rpc.Code 映射到对应的 HTTP 状态码。
var codeToHTTP = map[codes.Code]int{
	codes.OK:                 http.StatusOK,
	codes.Canceled:           499,
	codes.Unknown:            http.StatusInternalServerError,
	codes.InvalidArgument:    http.StatusBadRequest,
	codes.DeadlineExceeded:   http.StatusGatewayTimeout,
	codes.NotFound:           http.StatusNotFound,
	codes.AlreadyExists:      http.StatusConflict,
	codes.PermissionDenied:   http.StatusForbidden,
	codes.ResourceExhausted:  http.StatusTooManyRequests,
	codes.FailedPrecondition: http.StatusBadRequest,
	codes.Aborted:            http.StatusConflict,
	codes.OutOfRange:         http.StatusBadRequest,
	codes.Unimplemented:      http.StatusNotImplemented,
	codes.Internal:           http.StatusInternalServerError,
	codes.Unavailable:        http.StatusServiceUnavailable,
	codes.DataLoss:           http.StatusInternalServerError,
	codes.Unauthenticated:    http.StatusUnauthorized,
}

// FromRPCCode 返回 google.rpc.Code 对应的 HTTP 状态码。
// 未知 Code 返回 500。
func FromRPCCode(c codes.Code) int {
	if h, ok := codeToHTTP[c]; ok {
		return h
	}
	return http.StatusInternalServerError
}

// ToRPCCode 将 HTTP 状态码映射到最接近的 google.rpc.Code。
// 多个 RPC Code 共享同一 HTTP 状态码时（如 400、409、500），
// 取语义最通用的一个；业务层应直接使用 oops.Code 而非依赖此函数。
// 无法匹配时返回 codes.Unknown。
func ToRPCCode(httpCode int) codes.Code {
	switch httpCode {
	case http.StatusOK:
		return codes.OK
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusMethodNotAllowed:
		return codes.Unimplemented
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusRequestEntityTooLarge:
		return codes.ResourceExhausted
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case http.StatusNotImplemented:
		return codes.Unimplemented
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	case http.StatusGatewayTimeout:
		return codes.DeadlineExceeded
	case http.StatusInternalServerError:
		return codes.Internal
	default:
		return codes.Unknown
	}
}
