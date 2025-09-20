package service

import (
	"errors"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
)

var (
	ErrPageNotFound = errors.New("page not found")
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
