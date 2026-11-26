// Package i18n 提供服务对外错误消息与字段校验消息的本地化能力。
//
// 核心思路：
//   - 使用 pkg/rest.Code（13 个有限标准 HTTP 错误码）作为 [errors] 翻译主键，并支持按 Metadata.I18nSubkeys() 分层查找 [details.<CODE>]。
//   - 使用 pkg/validator 规则码作为 [validation] 翻译主键。
//   - 翻译结果通过 RFC 9457 Problem.Detail 与 FieldError.Message 暴露。
//
// 翻译表在启动时从 embed.FS 加载到内存后只读，无锁并发安全。
// 若默认 locale 缺失任一标准错误码或校验规则翻译，NewTranslator 启动时返回错误（fail-fast）。
package i18n

import "github.com/samber/do/v2"

// Package 将 Translator 注册到 DI 容器。
var Package = do.Package(
	do.Lazy(NewTranslator),
)
