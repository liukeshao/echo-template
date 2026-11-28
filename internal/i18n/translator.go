package i18n

import (
	"strings"

	"github.com/samber/do/v2"
	"golang.org/x/text/language"

	"github.com/liukeshao/go-rest-template/config"
)

// Translator 提供 locale 协商与消息模板渲染（只读无锁并发安全）。
type Translator interface {
	Match(acceptLanguage, queryLang string) language.Tag
	T(loc language.Tag, key string, vars map[string]string) (text string, used language.Tag, ok bool)
	Default() language.Tag
}

type translator struct {
	defaultTag language.Tag
	supported  []language.Tag
	matcher    language.Matcher
	messages   map[language.Tag]map[string]string
}

func NewTranslator(i do.Injector) (Translator, error) {
	cfg := do.MustInvoke[*config.Config](i)
	return newTranslator(cfg.I18n)
}

func newTranslator(cfg config.I18n) (Translator, error) {
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
		if text, ok := t.messages[t.defaultTag][key]; ok {
			return render(text, vars), t.defaultTag, true
		}
	}
	return "", language.Und, false
}

// render 使用单次扫描替换 {name} 占位符，避免二次展开；缺失变量保留原形。
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
