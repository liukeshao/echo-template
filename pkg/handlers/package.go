package handlers

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/httpruntime"
)

// Package 注册 HTTP 运行时、业务 Handler、文档处理器与 Server 到 DI 容器。
var Package = do.Package(
	do.Lazy(httpruntime.NewFromInjector),
	provideHandler(NewAuthHandler),
	provideHandler(NewBookHandler),
	provideHandler(NewMeHandler),
	do.Lazy(NewDocsHandler),
	do.Lazy(NewServer),
)
