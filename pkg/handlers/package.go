package handlers

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/middleware"
)

// Package registers all HTTP handler providers and the Echo instance
// into the DI container.
var Package = do.Package(
	do.Lazy(func(_ do.Injector) (*RouteRegistry, error) { return &RouteRegistry{}, nil }),
	do.Lazy(middleware.NewAuthMiddleware),
	provideHandler(NewAuthHandler),
	provideHandler(NewBookHandler),
	provideHandler(NewMeHandler),
	do.Lazy(NewDocsHandler),
	do.Lazy(NewServer),
)
