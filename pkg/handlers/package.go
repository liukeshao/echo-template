package handlers

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/rest"
)

// Package 注册 REST 运行时、业务处理器、文档处理器与 Server 到 DI 容器。
var Package = do.Package(
	do.Lazy(rest.NewFromInjector),
	do.Lazy(NewAuth),
	do.Lazy(NewBook),
	do.Lazy(NewMe),
	do.Lazy(NewDocs),
	do.Lazy(NewServer),
)
