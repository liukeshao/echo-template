package i18n

import (
	"fmt"

	"golang.org/x/text/language"

	"github.com/liukeshao/go-rest-template/config"
)

// parseSupported 解析支持的 BCP-47 locale，并将 defaultLocale 置于首位作为 language.Matcher 兜底。
func parseSupported(cfg config.I18n) (language.Tag, []language.Tag, error) {
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
