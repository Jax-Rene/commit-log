package service

import (
	"errors"
	"strings"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
)

var (
	ErrGalleryNotFound      = errors.New("gallery image not found")
	ErrGalleryImageMissing  = errors.New("gallery image is required")
	ErrGalleryStatusInvalid = errors.New("gallery status is invalid")
)

const (
	GalleryStatusPublished = "published"
	GalleryStatusDraft     = "draft"
)

// GalleryService handles gallery CRUD.
type GalleryService struct {
	db *gorm.DB
}

// GalleryFilter describes filters for listing gallery images.
type GalleryFilter struct {
	Search  string
	Status  string
	Page    int
	PerPage int
}

// GalleryListResult aggregates paginated gallery results.
type GalleryListResult struct {
	Items      []db.GalleryImage
	Total      int64
	TotalPages int
	Page       int
	PerPage    int
}

// GalleryInput represents fields accepted when creating or updating a gallery image.
type GalleryInput struct {
	Title       string
	Description string
	ImageURL    string
	ImageWidth  int
	ImageHeight int
	Status      string
	SortOrder   int
}

// NewGalleryService creates a GalleryService instance.
func NewGalleryService(gdb *gorm.DB) *GalleryService {
	return &GalleryService{db: gdb}
}

// ListAll returns all gallery images ordered by priority.
func (s *GalleryService) ListAll() ([]db.GalleryImage, error) {
	var items []db.GalleryImage
	if err := s.db.Order("sort_order desc").Order("created_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// List returns gallery images matching the filter.
func (s *GalleryService) List(filter GalleryFilter) (GalleryListResult, error) {
	result := GalleryListResult{
		Page:    normalizePage(filter.Page),
		PerPage: normalizePerPage(filter.PerPage, 12),
	}

	query := s.db.Model(&db.GalleryImage{})
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + search + "%"
		query = query.Where("title LIKE ? OR description LIKE ?", like, like)
	}

	if err := query.Count(&result.Total).Error; err != nil {
		return result, err
	}

	result.TotalPages = calculateTotalPages(result.Total, result.PerPage)
	offset := (result.Page - 1) * result.PerPage

	if err := query.Order("sort_order desc").Order("created_at desc").
		Limit(result.PerPage).
		Offset(offset).
		Find(&result.Items).Error; err != nil {
		return result, err
	}

	return result, nil
}

// ListPublished returns published gallery images with pagination.
func (s *GalleryService) ListPublished(page, perPage int) (GalleryListResult, error) {
	return s.List(GalleryFilter{
		Status:  GalleryStatusPublished,
		Page:    page,
		PerPage: perPage,
	})
}

// Get fetches a gallery image by id.
func (s *GalleryService) Get(id uint) (*db.GalleryImage, error) {
	var item db.GalleryImage
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGalleryNotFound
		}
		return nil, err
	}
	return &item, nil
}

// Create inserts a new gallery image.
func (s *GalleryService) Create(input GalleryInput) (*db.GalleryImage, error) {
	if err := validateGalleryInput(input); err != nil {
		return nil, err
	}

	sortOrder := input.SortOrder
	if sortOrder == 0 {
		order, err := s.nextSortOrder()
		if err != nil {
			return nil, err
		}
		sortOrder = order
	}

	item := db.GalleryImage{
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		ImageURL:    strings.TrimSpace(input.ImageURL),
		ImageWidth:  input.ImageWidth,
		ImageHeight: input.ImageHeight,
		Status:      normalizeGalleryStatus(input.Status),
		SortOrder:   sortOrder,
	}

	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// Update modifies an existing gallery image.
func (s *GalleryService) Update(id uint, input GalleryInput) (*db.GalleryImage, error) {
	if err := validateGalleryInput(input); err != nil {
		return nil, err
	}

	var item db.GalleryImage
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGalleryNotFound
		}
		return nil, err
	}

	item.Title = strings.TrimSpace(input.Title)
	item.Description = strings.TrimSpace(input.Description)
	item.ImageURL = strings.TrimSpace(input.ImageURL)
	item.ImageWidth = input.ImageWidth
	item.ImageHeight = input.ImageHeight
	item.Status = normalizeGalleryStatus(input.Status)
	item.SortOrder = input.SortOrder

	if err := s.db.Save(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// Delete removes a gallery image.
func (s *GalleryService) Delete(id uint) error {
	var item db.GalleryImage
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGalleryNotFound
		}
		return err
	}
	return s.db.Delete(&item).Error
}

func validateGalleryInput(input GalleryInput) error {
	if strings.TrimSpace(input.ImageURL) == "" {
		return ErrGalleryImageMissing
	}
	if input.ImageWidth <= 0 || input.ImageHeight <= 0 {
		return ErrGalleryImageMissing
	}
	status := normalizeGalleryStatus(input.Status)
	if status != GalleryStatusPublished && status != GalleryStatusDraft {
		return ErrGalleryStatusInvalid
	}
	return nil
}

func normalizeGalleryStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return GalleryStatusPublished
	}
	if status != GalleryStatusPublished && status != GalleryStatusDraft {
		return status
	}
	return status
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePerPage(perPage, fallback int) int {
	if perPage <= 0 {
		return fallback
	}
	return perPage
}

func calculateTotalPages(total int64, perPage int) int {
	if perPage <= 0 {
		return 1
	}
	if total == 0 {
		return 1
	}
	return int((total + int64(perPage) - 1) / int64(perPage))
}

func (s *GalleryService) nextSortOrder() (int, error) {
	var maxOrder int
	if err := s.db.Model(&db.GalleryImage{}).
		Select("COALESCE(MAX(sort_order), 0)").
		Scan(&maxOrder).Error; err != nil {
		return 0, err
	}
	return maxOrder + 1, nil
}
