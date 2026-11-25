package i18n

import (
	"context"

	"golang.org/x/text/language"
)

type localeKey struct{}

// WithLocale 在 context 中设置已协商好的 BCP-47 语言标签。
// 由 rest.Runtime.Locale 中间件在请求入口写入，供 rest.Runtime.WriteProblem 等下游消费。
func WithLocale(ctx context.Context, tag language.Tag) context.Context {
	return context.WithValue(ctx, localeKey{}, tag)
}

// Locale 从 context 中读取语言标签；未设置时返回 fallback。
// 采用 fallback 参数而非 ok 布尔，简化调用点。
func Locale(ctx context.Context, fallback language.Tag) language.Tag {
	if tag, ok := ctx.Value(localeKey{}).(language.Tag); ok {
		return tag
	}
	return fallback
}
