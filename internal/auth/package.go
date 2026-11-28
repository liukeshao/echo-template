package auth

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/go-rest-template/config"
)

var Package = do.Package(
	do.Lazy(func(i do.Injector) (*JWT, error) {
		cfg := do.MustInvoke[*config.Config](i)
		return NewJWT(cfg.Auth.JWT), nil
	}),
)
