// Package client aggregates DI packages for shared HTTP transport and all third-party API clients.
//
// To add a new external API integration:
//  1. Create internal/client/<apiname>/ with interface.go, client.go, errors.go, package.go.
//  2. Optionally add fine-grained i18n overrides in internal/i18n/messages/*.toml (e.g. [details.429]).
//  3. Add a field to config.Clients in config/config.go.
//  4. Append <apiname>.Package to the do.Package call below.
package client

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/internal/client/weather"
	"github.com/liukeshao/echo-template/pkg/httpclient"
)

func newSharedHTTPClient(_ do.Injector) (*httpclient.Client, error) {
	return httpclient.New(httpclient.Config{}), nil
}

// Package registers the shared HTTP client and all third-party API client providers into the DI container.
var Package = do.Package(
	do.Lazy(newSharedHTTPClient),
	weather.Package,
)
