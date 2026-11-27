package auth

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
)

// Package 向 DI 容器注册认证基础设施组件。
var Package = do.Package(
	do.Lazy(func(i do.Injector) (*JWT, error) {
		cfg := do.MustInvoke[*config.Config](i)
		return NewJWT(cfg.JWT), nil
	}),
)
