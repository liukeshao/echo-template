package httperr

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
)

// Status 是有限的标准 HTTP 错误状态码集合（底层即 HTTP 状态码整数，零重复映射），
// 本身实现 error 接口，可直接作为不可变零分配哨兵错误与 errors.Is / errors.AsType / oops.Wrap / oops.Join 配合使用。
type Status int

const (
	BadRequest           Status = http.StatusBadRequest            // 400
	Unauthorized         Status = http.StatusUnauthorized          // 401
	Forbidden            Status = http.StatusForbidden             // 403
	NotFound             Status = http.StatusNotFound              // 404
	MethodNotAllowed     Status = http.StatusMethodNotAllowed      // 405
	Conflict             Status = http.StatusConflict              // 409
	ContentTooLarge      Status = http.StatusRequestEntityTooLarge // 413
	UnsupportedMediaType Status = http.StatusUnsupportedMediaType  // 415
	TooManyRequests      Status = http.StatusTooManyRequests       // 429
	ClientClosedRequest  Status = 499                              // 499
	Internal             Status = http.StatusInternalServerError   // 500
	ServiceUnavailable   Status = http.StatusServiceUnavailable    // 503
	GatewayTimeout       Status = http.StatusGatewayTimeout        // 504
)

var allStatuses = []Status{
	BadRequest,
	Unauthorized,
	Forbidden,
	NotFound,
	MethodNotAllowed,
	Conflict,
	ContentTooLarge,
	UnsupportedMediaType,
	TooManyRequests,
	ClientClosedRequest,
	Internal,
	ServiceUnavailable,
	GatewayTimeout,
}

// AllStatuses 返回全部有限标准 HTTP 错误状态码副本，供 i18n 启动覆盖率门禁检查使用。
func AllStatuses() []Status {
	return slices.Clone(allStatuses)
}

// Valid 报告 s 是否处于合法 HTTP 错误状态码区间（400–599）。
func (s Status) Valid() bool { return s >= 400 && s <= 599 }

// Int 返回标准 int 型 HTTP 状态码。
func (s Status) Int() int { return int(s) }

// Key 返回状态码数字字符串（如 "404"），用于 i18n 查表（errors.404 / details.404.*）。
func (s Status) Key() string { return strconv.Itoa(int(s)) }

// Error 实现 error 接口，使每个 Status 常量本身即为标准哨兵错误。
func (s Status) Error() string { return fmt.Sprintf("%d %s", int(s), StatusTitle(int(s))) }

// String 实现 fmt.Stringer。
func (s Status) String() string { return s.Error() }
