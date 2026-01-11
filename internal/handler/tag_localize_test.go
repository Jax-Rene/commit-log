package handler

import (
	"testing"

	"github.com/commitlog/internal/db"
)

func TestLocalizedTagName(t *testing.T) {
	t.Parallel()

	tagBoth := db.Tag{
		Name: "产品",
		Translations: []db.TagTranslation{
			{Language: "zh", Name: "产品"},
			{Language: "en", Name: "Product"},
		},
	}

	tagOnlyZh := db.Tag{
		Name: "产品",
		Translations: []db.TagTranslation{
			{Language: "zh", Name: "产品"},
		},
	}

	cases := []struct {
		name     string
		tag      db.Tag
		language string
		want     string
	}{
		{name: "zh", tag: tagBoth, language: "zh", want: "产品"},
		{name: "en", tag: tagBoth, language: "en", want: "Product"},
		{name: "en_falls_back_to_zh", tag: tagOnlyZh, language: "en", want: "产品"},
		{name: "unknown_falls_back_to_zh", tag: tagBoth, language: "fr", want: "产品"},
		{name: "empty_language_uses_fallback", tag: tagBoth, language: "", want: "产品"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := localizedTagName(tc.tag, tc.language)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
