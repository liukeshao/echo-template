package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/config"
)

// newTestTranslator 构造包含默认 en + zh-CN 的 Translator；测试通用。
func newTestTranslator(t *testing.T) Translator {
	t.Helper()
	tr, err := newTranslator(config.I18nConfig{
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
			// Matcher 可能返回 base 或 region 变体；用 Base 比较更稳定。
			gotBase, _ := got.Base()
			wantBase, _ := tc.want.Base()
			assert.Equal(t, wantBase, gotBase, "got=%s want=%s", got, tc.want)
		})
	}
}

func TestTranslator_T_ErrorsWithMetadata(t *testing.T) {
	tr := newTestTranslator(t)

	zh := language.SimplifiedChinese
	text, used, ok := tr.T(zh, "errors.BOOK_NOT_FOUND", map[string]string{"book_id": "01ABC"})
	require.True(t, ok)
	assert.Equal(t, "未找到 ID 为 01ABC 的书籍。", text)
	assert.Equal(t, "zh-CN", used.String())

	text, _, ok = tr.T(language.English, "errors.BOOK_NOT_FOUND", map[string]string{"book_id": "01ABC"})
	require.True(t, ok)
	assert.Equal(t, "Book 01ABC was not found.", text)
}

func TestTranslator_T_FallbackToDefault(t *testing.T) {
	tr := newTestTranslator(t)

	// Japanese 未配置，应该回退到默认 en。
	jp := language.Japanese
	text, used, ok := tr.T(jp, "errors.VALIDATION_INVALID", nil)
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

	// 缺失 book_id 时占位符应保留，便于排查。
	text, _, ok := tr.T(language.English, "errors.BOOK_NOT_FOUND", nil)
	require.True(t, ok)
	assert.Contains(t, text, "{book_id}")
}

func TestTranslator_T_ValidationRuleKeys(t *testing.T) {
	tr := newTestTranslator(t)

	text, _, ok := tr.T(language.SimplifiedChinese, "validation.max_length",
		map[string]string{"field": "title", "max": "200"})
	require.True(t, ok)
	assert.Equal(t, "title 长度不能超过 200 个字符。", text)
}

func TestNewTranslator_MissingDefaultLocale(t *testing.T) {
	_, err := newTranslator(config.I18nConfig{
		DefaultLocale:    "",
		SupportedLocales: []string{"en"},
	})
	require.Error(t, err)
}

func TestNewTranslator_DefaultNotInSupported(t *testing.T) {
	_, err := newTranslator(config.I18nConfig{
		DefaultLocale:    "en",
		SupportedLocales: []string{"zh-CN"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be listed in supported_locales")
}

func TestNewTranslator_UnknownLocaleFile(t *testing.T) {
	_, err := newTranslator(config.I18nConfig{
		DefaultLocale:    "en",
		SupportedLocales: []string{"en", "fr"},
	})
	require.Error(t, err, "unknown locale file should fail-fast at startup")
}

func TestTranslator_MatchAndTranslate_RegionalVariants(t *testing.T) {
	tr := newTestTranslator(t)

	// zh-Hans / zh-SG should normalize to supported zh-CN and actually return Chinese text.
	for _, header := range []string{"zh-Hans", "zh-SG", "zh-CN,zh;q=0.9"} {
		loc := tr.Match(header, "")
		text, used, ok := tr.T(loc, "errors.BOOK_NOT_FOUND", map[string]string{"book_id": "01XYZ"})
		require.True(t, ok, "header=%s", header)
		assert.Equal(t, "zh-CN", used.String(), "header=%s", header)
		assert.Equal(t, "未找到 ID 为 01XYZ 的书籍。", text, "header=%s", header)
	}

	// en-US should normalize to supported en (not en-u-rg-uszzzz).
	loc := tr.Match("en-US,en;q=0.9", "")
	assert.Equal(t, "en", loc.String())
	text, used, ok := tr.T(loc, "errors.VALIDATION_INVALID", nil)
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
