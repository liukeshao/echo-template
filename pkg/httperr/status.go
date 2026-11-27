package httperr

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
)

// Status 表示标准 HTTP 错误状态码哨兵，可直接配合 errors.Is / oops.Wrap 使用。
type Status int

const (
	BadRequest           Status = http.StatusBadRequest
	Unauthorized         Status = http.StatusUnauthorized
	Forbidden            Status = http.StatusForbidden
	NotFound             Status = http.StatusNotFound
	MethodNotAllowed     Status = http.StatusMethodNotAllowed
	Conflict             Status = http.StatusConflict
	ContentTooLarge      Status = http.StatusRequestEntityTooLarge
	UnsupportedMediaType Status = http.StatusUnsupportedMediaType
	TooManyRequests      Status = http.StatusTooManyRequests
	ClientClosedRequest  Status = 499
	Internal             Status = http.StatusInternalServerError
	ServiceUnavailable   Status = http.StatusServiceUnavailable
	GatewayTimeout       Status = http.StatusGatewayTimeout
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

func AllStatuses() []Status {
	return slices.Clone(allStatuses)
}

func (s Status) Valid() bool    { return s >= 400 && s <= 599 }
func (s Status) Int() int       { return int(s) }
func (s Status) Key() string    { return strconv.Itoa(int(s)) }
func (s Status) Error() string  { return fmt.Sprintf("%d %s", int(s), StatusTitle(int(s))) }
func (s Status) String() string { return s.Error() }
