package weather

import (
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/pkg/httperr"
)

var errWeather = oops.In("weather").Tags("external_api")

// weatherUnavailable 将底层网络/超时/502-504 错误包装为 503 Service Unavailable。
func weatherUnavailable(err error, op string) error {
	return errWeather.
		With("op", op).
		Join(httperr.ServiceUnavailable, err)
}

// weatherRateLimited 构造上游限流（429 Too Many Requests）错误，附带 AIP-193 RetryInfo。
func weatherRateLimited(op, retryAfter string) error {
	return errWeather.
		With("op", op).
		With(httperr.MetaKey, httperr.RetryInfo{
			Service:    "weather",
			RetryAfter: retryAfter,
		}).
		Wrap(httperr.TooManyRequests)
}

// weatherInternal 包装非预期上游状态码或响应解码错误（500 Internal Server Error）。
func weatherInternal(err error, op string) error {
	return errWeather.
		With("op", op).
		Join(httperr.Internal, err)
}
