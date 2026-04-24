package i18n

import (
	"embed"
	"fmt"
	"sort"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/pkg/codes"
)

//go:embed messages/*.toml
var messagesFS embed.FS

// messagesDir 是嵌入文件的根目录前缀。
const messagesDir = "messages"

// loadMessages 从 embed.FS 读取所有已支持 locale 的翻译表。
//
// 文件命名约定：messages/<BCP-47>.toml。缺失文件直接返回错误（fail-fast）。
// 文件内为两层 TOML：[section].<key> = "template"，加载后扁平化为
// "section.key"。
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

// loadLocale 读取单个 locale 的 TOML 文件并扁平化为 "section.key" 形式。
func loadLocale(tag language.Tag) (map[string]string, error) {
	path := fmt.Sprintf("%s/%s.toml", messagesDir, tag.String())
	raw, err := messagesFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("i18n: read messages file %s: %w", path, err)
	}

	var parsed map[string]map[string]string
	if err := toml.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("i18n: parse messages file %s: %w", path, err)
	}

	flat := make(map[string]string)
	for section, entries := range parsed {
		for k, v := range entries {
			flat[section+"."+k] = v
		}
	}
	return flat, nil
}

// assertCoverage 保证默认 locale 中的 [errors] section 覆盖了所有已注册的业务错误码。
//
// 缺失任一注册 Code 的翻译 → 返回错误，由 NewTranslator 在启动时暴露给 DI 容器。
// 附加 locale 的缺失不会致命，运行期会回落到默认 locale。
func assertCoverage(defaultTable map[string]string, defaultTag language.Tag) error {
	if defaultTable == nil {
		return fmt.Errorf("i18n: default locale %s has no messages loaded", defaultTag)
	}

	var missing []string
	for _, code := range codes.RegisteredCodes() {
		key := "errors." + code.String()
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
