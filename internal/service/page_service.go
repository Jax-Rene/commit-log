package service

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/commitlog/internal/db"
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

// GetBySlug fetches a page for a given slug.
func (s *PageService) GetBySlug(slug string) (*db.Page, error) {
	var page db.Page
	if err := s.db.Where("slug = ?", slug).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}
	return &page, nil
}

// SaveAboutPage creates or updates the about page content.
func (s *PageService) SaveAboutPage(content string) (*db.Page, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, ErrPageContentMissing
	}

	summary := summarizeContent(trimmed)

	var page db.Page
	err := s.db.Where("slug = ?", "about").First(&page).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			page = db.Page{
				Slug:    "about",
				Title:   "About Me",
				Summary: summary,
				Content: trimmed,
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
		page.Title = "About Me"
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
