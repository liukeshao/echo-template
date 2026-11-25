package i18n

import (
	"fmt"

	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/config"
)

// parseSupported 将 config.I18nConfig 中的 locale 字符串解析为 BCP-47 标签。
//
// 约束：
//   - DefaultLocale 必须同时出现在 SupportedLocales 中。
//   - 所有 locale 字符串必须符合 BCP-47。
//
// 默认 locale 放在 supported 列表首位，language.NewMatcher 会把它当作 fallback。
func parseSupported(cfg config.I18nConfig) (language.Tag, []language.Tag, error) {
	if cfg.DefaultLocale == "" {
		return language.Und, nil, fmt.Errorf("i18n: default_locale is required")
	}
	if len(cfg.SupportedLocales) == 0 {
		return language.Und, nil, fmt.Errorf("i18n: supported_locales must not be empty")
	}

	defaultTag, err := language.Parse(cfg.DefaultLocale)
	if err != nil {
		return language.Und, nil, fmt.Errorf("i18n: invalid default_locale %q: %w", cfg.DefaultLocale, err)
	}

	tags := make([]language.Tag, 0, len(cfg.SupportedLocales))
	tags = append(tags, defaultTag)
	seen := map[language.Tag]struct{}{defaultTag: {}}
	hasDefault := false

	for _, s := range cfg.SupportedLocales {
		t, err := language.Parse(s)
		if err != nil {
			return language.Und, nil, fmt.Errorf("i18n: invalid supported locale %q: %w", s, err)
		}
		if t == defaultTag {
			hasDefault = true
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		tags = append(tags, t)
	}

	if !hasDefault {
		return language.Und, nil, fmt.Errorf("i18n: default_locale %q must be listed in supported_locales", cfg.DefaultLocale)
	}

	return defaultTag, tags, nil
}
