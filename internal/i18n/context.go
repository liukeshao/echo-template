package i18n

import (
	"context"

	"golang.org/x/text/language"
)

type localeKey struct{}

func WithLocale(ctx context.Context, tag language.Tag) context.Context {
	return context.WithValue(ctx, localeKey{}, tag)
}

func Locale(ctx context.Context, fallback language.Tag) language.Tag {
	if tag, ok := ctx.Value(localeKey{}).(language.Tag); ok {
		return tag
	}
	return fallback
}
