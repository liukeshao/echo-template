package handlers

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/middleware"
)

// Package registers all HTTP handler providers and the Echo instance
// into the DI container.
var Package = do.Package(
	do.Lazy(middleware.NewAuthMiddleware),
	do.Lazy(NewAuthHandler),
	do.Lazy(NewBookHandler),
	do.Lazy(NewMeHandler),
	do.Lazy(NewDocsHandler),
	do.Lazy(NewServer),
)
