package weather

import (
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/pkg/rest"
)

var errWeather = oops.In("weather").Tags("external_api", "weather")

// weatherUnavailable wraps a low-level transport/timeout error into an oops
// error classified as WEATHER_UNAVAILABLE (→ HTTP 503 Service Unavailable).
func weatherUnavailable(err error, op string) error {
	return errWeather.
		Code(codes.WeatherUnavailable).
		Wrapf(err, "%s", op)
}

// weatherRateLimited returns an oops error for HTTP 429 responses with AIP-193 RetryInfo metadata.
func weatherRateLimited(op, retryAfter string) error {
	return errWeather.
		Code(codes.WeatherRateLimited).
		With(rest.MetaKey, rest.RetryInfo{
			Service:    "weather",
			RetryAfter: retryAfter,
		}).
		Errorf("%s: rate limited by upstream", op)
}

// weatherInternal wraps an unexpected upstream or decoding error.
func weatherInternal(err error, op string) error {
	return errWeather.
		Code(codes.WeatherInternalError).
		Wrapf(err, "%s", op)
}
