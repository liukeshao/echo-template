package i18n

import (
	"embed"
	"fmt"
	"sort"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/validator"
)

//go:embed messages/*.toml
var messagesFS embed.FS

const messagesDir = "messages"

func loadMessages(supported []language.Tag) (map[language.Tag]map[string]string, error) {
	out := make(map[language.Tag]map[string]string, len(supported))
	for _, tag := range supported {
		table, err := loadLocale(tag)
		if err != nil {
			return nil, err
		}
		out[tag] = table
	}
	return out, nil
}

func loadLocale(tag language.Tag) (map[string]string, error) {
	path := fmt.Sprintf("%s/%s.toml", messagesDir, tag.String())
	raw, err := messagesFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("i18n: read messages file %s: %w", path, err)
	}

	var parsed map[string]any
	if err := toml.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("i18n: parse messages file %s: %w", path, err)
	}

	flat := make(map[string]string)
	if err := flattenTOML("", parsed, flat); err != nil {
		return nil, fmt.Errorf("i18n: flatten messages file %s: %w", path, err)
	}
	return flat, nil
}

func flattenTOML(prefix string, m map[string]any, dst map[string]string) error {
	for k, v := range m {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			dst[fullKey] = val
		case map[string]any:
			if err := flattenTOML(fullKey, val, dst); err != nil {
				return err
			}
		default:
			return fmt.Errorf("key %q has non-string value type %T", fullKey, v)
		}
	}
	return nil
}

// assertCoverage 校验默认 locale 完整覆盖全部 httperr 状态码与 validator 规则码。
func assertCoverage(defaultTable map[string]string, defaultTag language.Tag) error {
	if defaultTable == nil {
		return fmt.Errorf("i18n: default locale %s has no messages loaded", defaultTag)
	}

	var missing []string
	for _, s := range httperr.AllStatuses() {
		key := "errors." + s.Key()
		if _, ok := defaultTable[key]; !ok {
			missing = append(missing, key)
		}
	}
	for _, rule := range validator.RegisteredRules() {
		key := "validation." + rule.Key()
		if _, ok := defaultTable[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("i18n: default locale %s is missing translations for: %v", defaultTag, missing)
}
