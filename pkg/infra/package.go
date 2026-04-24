package infra

import "github.com/samber/do/v2"

// Package registers all infrastructure providers into the DI container.
//
// *HTTPClient — shared net/http client (default 10 s timeout)
// *DB         — raw SQL connection pool (Shutdowner)
// *ORM        — ent client wrapping *DB (Shutdowner); services invoke this directly
var Package = do.Package(
	do.Lazy(NewHTTPClient),
	do.Lazy(NewDB),
	do.Lazy(NewORM),
)
