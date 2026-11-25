// Package client aggregates DI packages for shared HTTP transport and all third-party API clients.
//
// To add a new external API integration:
//  1. Create internal/client/<apiname>/ with interface.go, client.go, errors.go, package.go.
//  2. Add the corresponding error codes in internal/codes/<apiname>.go.
//  3. Add a field to config.ClientsConfig in config/config.go.
//  4. Append <apiname>.Package to the do.Package call below.
package client

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/internal/client/weather"
	"github.com/liukeshao/echo-template/internal/httpclient"
)

// Package registers the shared HTTP client and all third-party API client providers into the DI container.
var Package = do.Package(
	httpclient.Package,
	weather.Package,
)
