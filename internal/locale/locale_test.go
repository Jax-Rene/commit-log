package locale

import "testing"

func TestNormalizeLanguage(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "zh", want: LanguageChinese},
		{input: "zh-CN", want: LanguageChinese},
		{input: "ZH_hans", want: LanguageChinese},
		{input: "en", want: LanguageEnglish},
		{input: "en-US", want: LanguageEnglish},
		{input: "fr", want: ""},
		{input: "", want: ""},
	}

	for _, tc := range cases {
		if got := NormalizeLanguage(tc.input); got != tc.want {
			t.Fatalf("NormalizeLanguage(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestLanguageFromCountryCode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "CN", want: LanguageChinese},
		{input: "cn", want: LanguageChinese},
		{input: "US", want: LanguageEnglish},
		{input: "", want: ""},
	}

	for _, tc := range cases {
		if got := LanguageFromCountryCode(tc.input); got != tc.want {
			t.Fatalf("LanguageFromCountryCode(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestLanguageFromAcceptLanguage(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "zh-CN,zh;q=0.9", want: LanguageChinese},
		{input: "en-US,en;q=0.9", want: LanguageEnglish},
		{input: "fr-FR,fr;q=0.9", want: ""},
		{input: "", want: ""},
	}

	for _, tc := range cases {
		if got := LanguageFromAcceptLanguage(tc.input); got != tc.want {
			t.Fatalf("LanguageFromAcceptLanguage(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestPreferenceForLanguage(t *testing.T) {
	pref := PreferenceForLanguage("en")
	if pref.Language != LanguageEnglish {
		t.Fatalf("expected language %q, got %q", LanguageEnglish, pref.Language)
	}
	if pref.Locale != "en_US" {
		t.Fatalf("expected locale en_US, got %q", pref.Locale)
	}
	if pref.HTMLLang != "en-US" {
		t.Fatalf("expected html lang en-US, got %q", pref.HTMLLang)
	}

	fallback := PreferenceForLanguage("")
	if fallback.Language != LanguageChinese {
		t.Fatalf("expected fallback language %q, got %q", LanguageChinese, fallback.Language)
	}
}

func TestPick(t *testing.T) {
	if got := Pick("en", "english", "chinese"); got != "english" {
		t.Fatalf("Pick(en) = %q, want %q", got, "english")
	}
	if got := Pick("zh", "english", "chinese"); got != "chinese" {
		t.Fatalf("Pick(zh) = %q, want %q", got, "chinese")
	}
	if got := Pick("fr", "english", "chinese"); got != "chinese" {
		t.Fatalf("Pick(fr) = %q, want %q", got, "chinese")
	}
}
