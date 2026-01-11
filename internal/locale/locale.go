package locale

import "strings"

const (
	LanguageChinese = "zh"
	LanguageEnglish = "en"
)

type Preference struct {
	Language string
	Locale   string
	HTMLLang string
}

func NormalizeLanguage(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "zh") || trimmed == "cn" {
		return LanguageChinese
	}
	if strings.HasPrefix(trimmed, "en") {
		return LanguageEnglish
	}
	return ""
}

func LanguageFromCountryCode(code string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(code))
	if trimmed == "" {
		return ""
	}
	if trimmed == "CN" {
		return LanguageChinese
	}
	return LanguageEnglish
}

func LanguageFromAcceptLanguage(header string) string {
	trimmed := strings.ToLower(strings.TrimSpace(header))
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "zh") {
		return LanguageChinese
	}
	if strings.Contains(trimmed, "en") {
		return LanguageEnglish
	}
	return ""
}

func PreferenceForLanguage(language string) Preference {
	normalized := NormalizeLanguage(language)
	if normalized == LanguageEnglish {
		return Preference{Language: LanguageEnglish, Locale: "en_US", HTMLLang: "en-US"}
	}
	return Preference{Language: LanguageChinese, Locale: "zh_CN", HTMLLang: "zh-CN"}
}
