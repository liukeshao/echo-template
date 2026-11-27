package service

import "github.com/samber/do/v2"

// Package 向 DI 容器注册全部领域服务。
var Package = do.Package(
	do.Lazy(NewAuth),
	do.Lazy(NewUser),
	do.Lazy(NewBook),
)
