package service

import "github.com/samber/do/v2"

var Package = do.Package(
	do.Lazy(NewAuth),
	do.Lazy(NewUser),
	do.Lazy(NewBook),
)
