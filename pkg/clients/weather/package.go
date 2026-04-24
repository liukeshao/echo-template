package weather

import "github.com/samber/do/v2"

// Package registers the Weather API client into the DI container.
//
// *Client — implements [API]; resolves config.Config and *infra.HTTPClient
var Package = do.Package(
	do.Lazy(NewWeatherClient),
)
