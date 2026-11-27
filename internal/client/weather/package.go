package weather

import "github.com/samber/do/v2"

var Package = do.Package(
	do.Lazy(NewClient),
	do.Bind[*Client, API](),
)
