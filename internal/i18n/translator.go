package i18n

import (
	"strings"

	"github.com/samber/do/v2"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/config"
)

// Translator 负责 locale 协商与翻译模板渲染。
//
// 实现约束：加载完成后所有读路径无锁、幂等，适合作为单例通过 DI 注入。
type Translator interface {
	// Match 返回与客户端偏好最匹配且服务端已支持的语言标签。
	// 优先级：queryLang > acceptLanguage > 默认 locale。
	Match(acceptLanguage, queryLang string) language.Tag

	// T 渲染给定 key 的消息模板。
	//   - 先在 loc 对应的翻译表查找；未命中时回落到默认 locale。
	//   - vars 用于 {name} 占位符替换；缺失变量保留原形 `{name}` 便于排查。
	//   - used 返回实际命中的 locale；ok=false 表示所有 locale 均未命中。
	T(loc language.Tag, key string, vars map[string]string) (text string, used language.Tag, ok bool)

	// Default 返回系统默认 locale，供缺失 locale 时兜底使用。
	Default() language.Tag
}

// translator 是 Translator 的默认实现。
//
// messages: key = 翻译键（如 "errors.BOOK_NOT_FOUND"），value = 模板字符串。
// 每个 supportedLocale 对应一张独立表，缺失键走默认 locale 兜底。
type translator struct {
	defaultTag language.Tag
	supported  []language.Tag
	matcher    language.Matcher
	messages   map[language.Tag]map[string]string
}

// NewTranslator 从 DI 容器构造 Translator：
//   - 读取 I18n 配置确定默认 locale 与已支持 locale 列表。
//   - 从 embed.FS 加载 messages/*.toml 到内存。
//   - 校验所有 internal/codes 中注册的 Code 在默认 locale 都有对应翻译（fail-fast）。
func NewTranslator(i do.Injector) (Translator, error) {
	cfg := do.MustInvoke[*config.Config](i)
	return newTranslator(cfg.I18n)
}

// newTranslator 是 NewTranslator 的纯函数内核，便于直接在测试中构造。
func newTranslator(cfg config.I18nConfig) (Translator, error) {
	defaultTag, supported, err := parseSupported(cfg)
	if err != nil {
		return nil, err
	}

	messages, err := loadMessages(supported)
	if err != nil {
		return nil, err
	}

	if err := assertCoverage(messages[defaultTag], defaultTag); err != nil {
		return nil, err
	}

	return &translator{
		defaultTag: defaultTag,
		supported:  supported,
		matcher:    language.NewMatcher(supported),
		messages:   messages,
	}, nil
}

func (t *translator) Default() language.Tag { return t.defaultTag }

func (t *translator) Match(acceptLanguage, queryLang string) language.Tag {
	if q := strings.TrimSpace(queryLang); q != "" {
		if tag, err := language.Parse(q); err == nil {
			_, idx, _ := t.matcher.Match(tag)
			return t.supported[idx]
		}
	}
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil || len(tags) == 0 {
		return t.defaultTag
	}
	_, idx, _ := t.matcher.Match(tags...)
	return t.supported[idx]
}

func (t *translator) T(loc language.Tag, key string, vars map[string]string) (string, language.Tag, bool) {
	// 先尝试精确命中；若 loc 并非已加载的具体 locale（例如调用方传入 zh-Hans
	// 而我们的表键为 zh-CN），通过 matcher 归一化到最接近的支持 locale 再查。
	resolved, ok := t.messages[loc]
	if !ok {
		_, idx, _ := t.matcher.Match(loc)
		loc = t.supported[idx]
		resolved = t.messages[loc]
	}
	if resolved != nil {
		if text, ok := resolved[key]; ok {
			return render(text, vars), loc, true
		}
	}
	if loc != t.defaultTag {
		if text, ok := t.lookup(t.defaultTag, key); ok {
			return render(text, vars), t.defaultTag, true
		}
	}
	return "", language.Und, false
}

// lookup 在给定 locale 的翻译表中查找 key，未命中返回 ok=false。
func (t *translator) lookup(loc language.Tag, key string) (string, bool) {
	table, ok := t.messages[loc]
	if !ok {
		return "", false
	}
	text, ok := table[key]
	return text, ok
}

// render 用 vars 替换模板中的 {name} 占位符。
// 使用单次扫描的 strings.Replacer，避免多次替换导致的非确定性与占位符二次展开。
// 缺失变量保留原形 `{name}` 便于 QA 快速发现翻译/代码不一致。
func render(template string, vars map[string]string) string {
	if len(vars) == 0 || !strings.ContainsRune(template, '{') {
		return template
	}
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(template)
}
