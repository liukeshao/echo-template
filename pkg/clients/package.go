// Package clients aggregates DI packages for all third-party API clients.
//
// To add a new external API integration:
//  1. Create pkg/clients/<apiname>/ with interface.go, client.go, errors.go, package.go.
//  2. Add the corresponding error codes in pkg/codes/<apiname>.go.
//  3. Add a field to config.ClientsConfig in config/config.go.
//  4. Append <apiname>.Package to the do.Package call below.
package clients

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/clients/weather"
)

// Package registers all third-party API client providers into the DI container.
var Package = do.Package(
	weather.Package,
)
