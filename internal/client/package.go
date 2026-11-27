package client

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/internal/client/weather"
	"github.com/liukeshao/echo-template/pkg/httpclient"
)

func newSharedHTTPClient(_ do.Injector) (*httpclient.Client, error) {
	return httpclient.New(httpclient.Config{}), nil
}

var Package = do.Package(
	do.Lazy(newSharedHTTPClient),
	weather.Package,
)
