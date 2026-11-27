package weather

import "github.com/samber/do/v2"

// Package 将天气 API 具体客户端注册到 DI 并绑定至 API 接口。
var Package = do.Package(
	do.Lazy(NewClient),
	do.Bind[*Client, API](),
)
