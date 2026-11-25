// Package i18n 提供服务对外错误消息与字段校验消息的本地化能力。
//
// 核心思路：
//   - 使用 internal/codes.Code 作为错误翻译 key 的稳定主键（key = "errors.<CODE>"）。
//   - 使用稳定键 "validation.<rule>" 作为字段校验消息的翻译键。
//   - 翻译结果通过 RFC 9457 Problem.Detail 与 FieldError.Message 暴露。
//
// 翻译表在启动时从 embed.FS 加载到内存后只读，无锁并发安全。
// 若默认 locale 缺失任一已注册 Code 的翻译，loader 启动时 panic（fail-fast）。
package i18n

import "github.com/samber/do/v2"

// Package 将 Translator 注册到 DI 容器。
var Package = do.Package(
	do.Lazy(NewTranslator),
)
