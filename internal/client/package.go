// Package client 聚合共享出站 HTTP 客户端与全部第三方 API 客户端的 DI 注册。
//
// 新增第三方 API 集成步骤：
//  1. 新建 internal/client/<apiname>/（含 interface.go、client.go、errors.go、package.go）。
//  2. 按需在 internal/i18n/messages/*.toml 补充细化文案（如 [details.429]）。
//  3. 在 config/config.go 的 Clients 结构体中添加字段，并在 config.toml 补充配置块。
//  4. 将 <apiname>.Package 追加到下方 do.Package。
package client

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/internal/client/weather"
	"github.com/liukeshao/echo-template/pkg/httpclient"
)

func newSharedHTTPClient(_ do.Injector) (*httpclient.Client, error) {
	return httpclient.New(httpclient.Config{}), nil
}

// Package 将共享 HTTP 客户端与所有第三方 API 客户端注册到 DI 容器。
var Package = do.Package(
	do.Lazy(newSharedHTTPClient),
	weather.Package,
)
