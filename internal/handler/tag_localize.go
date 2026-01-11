package handler

import (
	"strings"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/locale"
)

func localizedTagName(tag db.Tag, language string) string {
	fallback := strings.TrimSpace(tag.Name)

	normalized := locale.NormalizeLanguage(language)
	if normalized == "" || len(tag.Translations) == 0 {
		return fallback
	}

	var zhName string
	var enName string
	for _, translation := range tag.Translations {
		lang := locale.NormalizeLanguage(translation.Language)
		name := strings.TrimSpace(translation.Name)
		if name == "" || lang == "" {
			continue
		}
		if lang == normalized {
			return name
		}
		switch lang {
		case locale.LanguageChinese:
			if zhName == "" {
				zhName = name
			}
		case locale.LanguageEnglish:
			if enName == "" {
				enName = name
			}
		}
	}

	switch normalized {
	case locale.LanguageChinese:
		if zhName != "" {
			return zhName
		}
		if enName != "" {
			return enName
		}
	case locale.LanguageEnglish:
		if enName != "" {
			return enName
		}
		if zhName != "" {
			return zhName
		}
	default:
		if zhName != "" {
			return zhName
		}
		if enName != "" {
			return enName
		}
	}

	return fallback
}

func localizeTagSliceInPlace(tags []db.Tag, language string) map[uint]string {
	names := make(map[uint]string, len(tags))
	for i := range tags {
		displayName := localizedTagName(tags[i], language)
		if displayName == "" {
			displayName = strings.TrimSpace(tags[i].Name)
		}
		if displayName != "" {
			tags[i].Name = displayName
			names[tags[i].ID] = displayName
		}
	}
	return names
}

func localizePostTagNamesInPlace(posts []db.Post, tagNamesByID map[uint]string) {
	if len(posts) == 0 || len(tagNamesByID) == 0 {
		return
	}
	for i := range posts {
		for j := range posts[i].Tags {
			if name, ok := tagNamesByID[posts[i].Tags[j].ID]; ok && strings.TrimSpace(name) != "" {
				posts[i].Tags[j].Name = name
			}
		}
	}
}

func localizePublicationTagNamesInPlace(publications []db.PostPublication, language string) {
	if len(publications) == 0 {
		return
	}
	for i := range publications {
		if len(publications[i].Tags) == 0 {
			continue
		}
		localizeTagSliceInPlace(publications[i].Tags, language)
	}
}
