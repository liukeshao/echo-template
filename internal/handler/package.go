package handler

import "github.com/samber/do/v2"

// Package 注册全部业务领域处理器到 DI 容器。
var Package = do.Package(
	do.Lazy(NewAuth),
	do.Lazy(NewBook),
	do.Lazy(NewMe),
)
