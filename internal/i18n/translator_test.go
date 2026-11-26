package i18n

import (
	"context"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func newTestTranslator(t *testing.T) Translator {
	t.Helper()
	tr, err := newTranslator(config.I18n{
		DefaultLocale:    "en",
		SupportedLocales: []string{"en", "zh-CN"},
	})
	require.NoError(t, err, "newTranslator must succeed with bundled messages")
	return tr
}

func TestTranslator_Match(t *testing.T) {
	tr := newTestTranslator(t)

	cases := []struct {
		name   string
		header string
		query  string
		want   language.Tag
	}{
		{"query overrides header", "en", "zh-CN", language.SimplifiedChinese},
		{"accept-language chinese", "zh-CN,zh;q=0.9", "", language.SimplifiedChinese},
		{"accept-language english", "en-US,en;q=0.9", "", language.English},
		{"unknown falls back to default", "fr-FR", "", language.English},
		{"empty returns default", "", "", language.English},
		{"invalid query falls through to header", "zh-CN", "!!!bad", language.SimplifiedChinese},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tr.Match(tc.header, tc.query)
			gotBase, _ := got.Base()
			wantBase, _ := tc.want.Base()
			assert.Equal(t, wantBase, gotBase, "got=%s want=%s", got, tc.want)
		})
	}
}

func TestTranslator_T_HierarchicalDetailsAndMetadata(t *testing.T) {
	tr := newTestTranslator(t)

	zh := language.SimplifiedChinese
	text, used, ok := tr.T(zh, "details.NOT_FOUND.book", map[string]string{"resource_name": "01ABC"})
	require.True(t, ok)
	assert.Equal(t, "未找到 ID 为 01ABC 的书籍。", text)
	assert.Equal(t, "zh-CN", used.String())

	text, _, ok = tr.T(language.English, "details.NOT_FOUND.book", map[string]string{"resource_name": "01ABC"})
	require.True(t, ok)
	assert.Equal(t, "Book 01ABC was not found.", text)

	// Nested dotted key flattened cleanly: details.CONFLICT.user.email
	text, _, ok = tr.T(zh, "details.CONFLICT.user.email", map[string]string{"resource_name": "a@b.com"})
	require.True(t, ok)
	assert.Equal(t, "邮箱 a@b.com 已被使用。", text)
}

func TestTranslator_T_FallbackToDefault(t *testing.T) {
	tr := newTestTranslator(t)

	jp := language.Japanese
	text, used, ok := tr.T(jp, "details.BAD_REQUEST.validation", nil)
	require.True(t, ok)
	assert.Equal(t, "Request validation failed.", text)
	assert.Equal(t, language.English, used)
}

func TestTranslator_T_UnknownKey(t *testing.T) {
	tr := newTestTranslator(t)

	_, _, ok := tr.T(language.English, "errors.DEFINITELY_NOT_REGISTERED", nil)
	assert.False(t, ok)
}

func TestTranslator_T_MissingVarKeepsPlaceholder(t *testing.T) {
	tr := newTestTranslator(t)

	text, _, ok := tr.T(language.English, "details.NOT_FOUND.book", nil)
	require.True(t, ok)
	assert.Contains(t, text, "{resource_name}")
}

func TestTranslator_T_ValidationRuleKeys(t *testing.T) {
	tr := newTestTranslator(t)

	text, _, ok := tr.T(language.SimplifiedChinese, "validation.max_length",
		map[string]string{"field": "title", "max": "200"})
	require.True(t, ok)
	assert.Equal(t, "title 长度不能超过 200 个字符。", text)
}

func TestNewTranslator_MissingDefaultLocale(t *testing.T) {
	_, err := newTranslator(config.I18n{
		DefaultLocale:    "",
		SupportedLocales: []string{"en"},
	})
	require.Error(t, err)
}

func TestNewTranslator_DefaultNotInSupported(t *testing.T) {
	_, err := newTranslator(config.I18n{
		DefaultLocale:    "en",
		SupportedLocales: []string{"zh-CN"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be listed in supported_locales")
}

func TestNewTranslator_UnknownLocaleFile(t *testing.T) {
	_, err := newTranslator(config.I18n{
		DefaultLocale:    "en",
		SupportedLocales: []string{"en", "fr"},
	})
	require.Error(t, err, "unknown locale file should fail-fast at startup")
}

func TestTranslator_MatchAndTranslate_RegionalVariants(t *testing.T) {
	tr := newTestTranslator(t)

	for _, header := range []string{"zh-Hans", "zh-SG", "zh-CN,zh;q=0.9"} {
		loc := tr.Match(header, "")
		text, used, ok := tr.T(loc, "details.NOT_FOUND.book", map[string]string{"resource_name": "01XYZ"})
		require.True(t, ok, "header=%s", header)
		assert.Equal(t, "zh-CN", used.String(), "header=%s", header)
		assert.Equal(t, "未找到 ID 为 01XYZ 的书籍。", text, "header=%s", header)
	}

	loc := tr.Match("en-US,en;q=0.9", "")
	assert.Equal(t, "en", loc.String())
	text, used, ok := tr.T(loc, "details.BAD_REQUEST.validation", nil)
	require.True(t, ok)
	assert.Equal(t, "en", used.String())
	assert.Equal(t, "Request validation failed.", text)
}

func TestRender_SinglePassNoReexpansion(t *testing.T) {
	got := render("{a} and {b}", map[string]string{
		"a": "{b}",
		"b": "VALUE",
	})
	assert.Equal(t, "{b} and VALUE", got)
}

var placeholderRE = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

func extractPlaceholders(tmpl string) []string {
	matches := placeholderRE.FindAllStringSubmatch(tmpl, -1)
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if _, ok := seen[m[1]]; !ok {
			seen[m[1]] = struct{}{}
			out = append(out, m[1])
		}
	}
	slices.Sort(out)
	return out
}

// TestMessages_LocaleKeyAndPlaceholderParity is a template invariant gate:
//   - every canonical HTTP error code and validation rule must exist in BOTH en.toml and zh-CN.toml;
//   - both locale files must define the exact same set of keys (including [details.*]);
//   - every key must reference the exact same {placeholder} set across locales.
func TestMessages_LocaleKeyAndPlaceholderParity(t *testing.T) {
	enTable, err := loadLocale(language.English)
	require.NoError(t, err)
	zhTable, err := loadLocale(language.MustParse("zh-CN"))
	require.NoError(t, err)

	for k, enTmpl := range enTable {
		zhTmpl, ok := zhTable[k]
		if assert.True(t, ok, "zh-CN.toml missing key %q present in en.toml", k) {
			assert.Equal(t, extractPlaceholders(enTmpl), extractPlaceholders(zhTmpl),
				"placeholder mismatch for key %q (en=%q, zh-CN=%q)", k, enTmpl, zhTmpl)
		}
	}
	for k := range zhTable {
		assert.Contains(t, enTable, k, "en.toml missing key %q present in zh-CN.toml", k)
	}

	for _, c := range rest.AllCodes() {
		key := "errors." + c.String()
		assert.Contains(t, enTable, key)
		assert.Contains(t, zhTable, key)
	}

	for _, rule := range validator.RegisteredRules() {
		key := "validation." + rule
		assert.Contains(t, enTable, key)
		assert.Contains(t, zhTable, key)
	}
}

func TestAssertCoverage_DetectsMissingCodeAndRule(t *testing.T) {
	err := assertCoverage(map[string]string{}, language.English)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing translations")
	assert.Contains(t, err.Error(), "errors.NOT_FOUND")
	assert.Contains(t, err.Error(), "validation.required")
	assert.Error(t, assertCoverage(nil, language.English))
}

func TestLocaleContext(t *testing.T) {
	assert.Equal(t, language.English, Locale(context.Background(), language.English))

	ctx := WithLocale(context.Background(), language.SimplifiedChinese)
	assert.Equal(t, language.SimplifiedChinese, Locale(ctx, language.English))
}
