package weather

import (
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/pkg/httperr"
)

var errWeather = oops.In("weather").Tags("external_api")

func weatherUnavailable(err error, op string) error {
	return errWeather.
		With("op", op).
		Join(httperr.ServiceUnavailable, err)
}

func weatherRateLimited(op, retryAfter string) error {
	return errWeather.
		With("op", op).
		With(httperr.MetaKey, httperr.RetryInfo{
			Service:    "weather",
			RetryAfter: retryAfter,
		}).
		Wrap(httperr.TooManyRequests)
}

func weatherInternal(err error, op string) error {
	return errWeather.
		With("op", op).
		Join(httperr.Internal, err)
}
