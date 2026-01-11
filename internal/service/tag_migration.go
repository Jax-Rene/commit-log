package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/locale"
	"gorm.io/gorm"
)

// BackfillTagTranslations creates missing tag translation rows using Tag.Name as the default display name.
// It is safe to run multiple times (idempotent for the requested languages).
func (s *TagService) BackfillTagTranslations(languages []string) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("tag service is not initialized")
	}

	normalizedLanguages := make([]string, 0, len(languages))
	seen := make(map[string]struct{}, len(languages))
	for _, raw := range languages {
		normalized := locale.NormalizeLanguage(raw)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		normalizedLanguages = append(normalizedLanguages, normalized)
	}
	if len(normalizedLanguages) == 0 {
		return 0, errors.New("no valid languages provided")
	}

	createdCount := 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var tags []db.Tag
		if err := tx.Select("id, name").Find(&tags).Error; err != nil {
			return fmt.Errorf("list tags: %w", err)
		}

		for _, tag := range tags {
			name := strings.TrimSpace(tag.Name)
			if name == "" {
				continue
			}
			for _, language := range normalizedLanguages {
				var existing db.TagTranslation
				err := tx.Where("tag_id = ? AND language = ?", tag.ID, language).First(&existing).Error
				switch {
				case err == nil:
					continue
				case errors.Is(err, gorm.ErrRecordNotFound):
					translation := db.TagTranslation{
						TagID:    tag.ID,
						Language: language,
						Name:     name,
					}
					if err := tx.Create(&translation).Error; err != nil {
						return fmt.Errorf("create tag translation (tag_id=%d, language=%s): %w", tag.ID, language, err)
					}
					createdCount++
				default:
					return fmt.Errorf("check tag translation (tag_id=%d, language=%s): %w", tag.ID, language, err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}
	return createdCount, nil
}
