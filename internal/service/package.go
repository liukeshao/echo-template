package service

import "github.com/samber/do/v2"

// Package registers all service providers into the DI container.
var Package = do.Package(
	do.Lazy(NewAuth),
	do.Lazy(NewMe),
	do.Lazy(NewBook),
)
