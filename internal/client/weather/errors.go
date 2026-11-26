package weather

import (
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/pkg/rest"
)

var errWeather = oops.In("weather").Tags("external_api")

// weatherUnavailable wraps a low-level transport/timeout error into an oops
// error classified as SERVICE_UNAVAILABLE (→ HTTP 503).
func weatherUnavailable(err error, op string) error {
	return errWeather.
		Code(rest.CodeServiceUnavailable).
		With("op", op).
		Join(rest.CodeServiceUnavailable, err)
}

// weatherRateLimited returns an oops error for HTTP 429 responses with AIP-193 RetryInfo metadata.
func weatherRateLimited(op, retryAfter string) error {
	return errWeather.
		Code(rest.CodeTooManyRequests).
		With("op", op).
		With(rest.MetaKey, rest.RetryInfo{
			Service:    "weather",
			RetryAfter: retryAfter,
		}).
		Wrap(rest.CodeTooManyRequests)
}

// weatherInternal wraps an unexpected upstream or decoding error (→ HTTP 500).
func weatherInternal(err error, op string) error {
	return errWeather.
		Code(rest.CodeInternal).
		With("op", op).
		Join(rest.CodeInternal, err)
}
