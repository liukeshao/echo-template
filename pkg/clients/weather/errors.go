package weather

import (
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/pkg/codes"
)

// weatherUnavailable wraps a low-level transport/timeout error into an oops
// error classified as WEATHER_UNAVAILABLE (→ HTTP 503 Service Unavailable).
// op should be a low-cardinality operation name (e.g. "GetCurrent").
func weatherUnavailable(err error, op string) error {
	return oops.
		Code(codes.WeatherUnavailable.String()).
		Tags("external_api", "weather").
		Wrapf(err, "%s", op)
}

// weatherRateLimited returns an oops error for HTTP 429 responses.
// The public message is safe to surface to the caller.
func weatherRateLimited(op string) error {
	return oops.
		Code(codes.WeatherRateLimited.String()).
		Tags("external_api", "weather").
		Public("Weather API rate limit exceeded. Please retry later.").
		Errorf("%s: rate limited by upstream", op)
}

// weatherInternal wraps an unexpected upstream or decoding error.
// Details are intentionally hidden from the public message.
func weatherInternal(err error, op string) error {
	return oops.
		Code(codes.WeatherInternal.String()).
		Tags("external_api", "weather").
		Wrapf(err, "%s", op)
}
