// Package i18n 提供基于 embed.FS TOML 的无锁并发安全错误与校验文案本地化。
package i18n

import "github.com/samber/do/v2"

var Package = do.Package(
	do.Lazy(NewTranslator),
)
