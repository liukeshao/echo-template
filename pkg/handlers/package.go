package handlers

import "github.com/samber/do/v2"

// Package registers all HTTP handler providers and the Echo instance
// into the DI container.
var Package = do.Package(
	provideHandler(NewAuthHandler),
	provideHandler(NewBookHandler),
	provideHandler(NewMeHandler),
	do.Lazy(NewDocsHandler),
	do.Lazy(NewServer),
)
