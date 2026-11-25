package database

import "github.com/samber/do/v2"

// Package registers database and ORM providers into the DI container.
//
// *DB  — raw SQL connection pool (Shutdowner)
// *ORM — ent client wrapping *DB (Shutdowner); services invoke this directly
var Package = do.Package(
	do.Lazy(NewDB),
	do.Lazy(NewORM),
)
