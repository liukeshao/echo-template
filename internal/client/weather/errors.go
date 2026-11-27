package weather

import (
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/pkg/httperr"
)

var errWeather = oops.In("weather").Tags("external_api")

// weatherUnavailable wraps a low-level transport/timeout error into an oops
// error classified as 503 Service Unavailable.
func weatherUnavailable(err error, op string) error {
	return errWeather.
		With("op", op).
		Join(httperr.ServiceUnavailable, err)
}

// weatherRateLimited returns an oops error for HTTP 429 responses with AIP-193 RetryInfo metadata.
func weatherRateLimited(op, retryAfter string) error {
	return errWeather.
		With("op", op).
		With(httperr.MetaKey, httperr.RetryInfo{
			Service:    "weather",
			RetryAfter: retryAfter,
		}).
		Wrap(httperr.TooManyRequests)
}

// weatherInternal wraps an unexpected upstream or decoding error (→ HTTP 500).
func weatherInternal(err error, op string) error {
	return errWeather.
		With("op", op).
		Join(httperr.Internal, err)
}
