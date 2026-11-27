package database

import "github.com/samber/do/v2"

// Package 将数据库连接池（*DB）与 Ent 客户端（*ORM）注册到 DI 容器。
var Package = do.Package(
	do.Lazy(NewDB),
	do.Lazy(NewORM),
)
