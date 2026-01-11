package service

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/locale"
	"gorm.io/gorm"
)

var (
	ErrPageNotFound       = errors.New("page not found")
	ErrPageContentMissing = errors.New("page content is required")
)

// PageService provides access to static pages such as About.
type PageService struct {
	db *gorm.DB
}

// NewPageService returns a new PageService instance.
func NewPageService(gdb *gorm.DB) *PageService {
	return &PageService{db: gdb}
}

// GetBySlug fetches a page for a given slug and language.
func (s *PageService) GetBySlug(slug, language string) (*db.Page, error) {
	normalized := normalizePageLanguage(language)
	var page db.Page
	if err := s.db.Where("slug = ? AND language = ?", slug, normalized).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}
	return &page, nil
}

// SaveAboutPage creates or updates the about page content.
func (s *PageService) SaveAboutPage(content, language string) (*db.Page, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, ErrPageContentMissing
	}

	summary := summarizeContent(trimmed)
	normalized := normalizePageLanguage(language)
	defaultTitle := defaultAboutTitle(normalized)

	var page db.Page
	err := s.db.Where("slug = ? AND language = ?", "about", normalized).First(&page).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			page = db.Page{
				Slug:     "about",
				Title:    defaultTitle,
				Summary:  summary,
				Content:  trimmed,
				Language: normalized,
			}
			if err := s.db.Create(&page).Error; err != nil {
				return nil, err
			}
			return &page, nil
		}
		return nil, err
	}

	page.Content = trimmed
	page.Summary = summary
	if strings.TrimSpace(page.Title) == "" {
		page.Title = defaultTitle
	}

	if err := s.db.Save(&page).Error; err != nil {
		return nil, err
	}

	return &page, nil
}

func summarizeContent(markdown string) string {
	plain := markdown
	replacer := strings.NewReplacer(
		"#", " ",
		"*", " ",
		"`", " ",
		"_", " ",
		">", " ",
		"[", " ",
		"]", " ",
		"(", " ",
		")", " ",
	)
	plain = replacer.Replace(plain)
	plain = strings.Join(strings.Fields(plain), " ")
	if plain == "" {
		return ""
	}

	const limit = 120
	if utf8.RuneCountInString(plain) <= limit {
		return plain
	}

	runes := []rune(plain)
	return string(runes[:limit]) + "…"
}

func normalizePageLanguage(language string) string {
	normalized := locale.NormalizeLanguage(language)
	if normalized == "" {
		return locale.LanguageChinese
	}
	return normalized
}

func defaultAboutTitle(language string) string {
	if language == locale.LanguageEnglish {
		return "About Me"
	}
	return "关于我"
}
