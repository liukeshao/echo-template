package weather

import "github.com/samber/do/v2"

// Package registers the Weather API client into the DI container.
//
// *Client — concrete client; bound to [API] so services depend only on [API]
var Package = do.Package(
	do.Lazy(NewWeatherClient),
	do.Bind[*Client, API](),
)
