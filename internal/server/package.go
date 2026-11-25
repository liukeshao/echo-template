package server

import "github.com/samber/do/v2"

// Package 注册文档处理器与 HTTP Server 到 DI 容器。
var Package = do.Package(
	do.Lazy(NewDocs),
	do.Lazy(newServer),
)
