package service

import (
	"errors"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
)

var (
	ErrPostNotFound  = errors.New("post not found")
	ErrCoverRequired = errors.New("cover image is required")
	ErrCoverInvalid  = errors.New("cover dimensions are invalid")
)

// PostService wraps post related database operations.
type PostService struct {
	db *gorm.DB
}

// PostFilter describes filters for listing posts.
type PostFilter struct {
	Search    string
	Status    string
	TagNames  []string
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	PerPage   int
}

// PostListResult aggregates paginated list data and counters.
type PostListResult struct {
	Posts          []db.Post
	Total          int64
	PublishedCount int64
	DraftCount     int64
	TotalPages     int
	Page           int
	PerPage        int
}

// PostInput represents fields accepted when creating or updating a post.
type PostInput struct {
	Title       string
	Content     string
	Summary     string
	Status      string
	TagIDs      []uint
	UserID      uint
	CoverURL    string
	CoverWidth  int
	CoverHeight int
}

// NewPostService creates a PostService instance.
func NewPostService(gdb *gorm.DB) *PostService {
	return &PostService{db: gdb}
}

// ListAll returns all posts ordered by created time descending.
func (s *PostService) ListAll() ([]db.Post, error) {
	var posts []db.Post
	if err := s.db.Preload("Tags").Order("created_at desc").Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

// Get fetches a post by id with tags preloaded.
func (s *PostService) Get(id uint) (*db.Post, error) {
	var post db.Post
	if err := s.db.Preload("Tags").First(&post, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	return &post, nil
}

// Create persists a post and associates tags in a transaction.
func (s *PostService) Create(input PostInput) (*db.Post, error) {
	coverURL, coverWidth, coverHeight, err := normalizeCover(input)
	if err != nil {
		return nil, err
	}

	post := db.Post{
		Title:       strings.TrimSpace(input.Title),
		Content:     input.Content,
		Summary:     input.Summary,
		Status:      input.Status,
		UserID:      input.UserID,
		CoverURL:    coverURL,
		CoverWidth:  coverWidth,
		CoverHeight: coverHeight,
	}

	return s.saveWithTags(&post, input.TagIDs)
}

// Update applies updates to an existing post.
func (s *PostService) Update(id uint, input PostInput) (*db.Post, error) {
	var existing db.Post
	if err := s.db.First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	coverURL, coverWidth, coverHeight, err := normalizeCover(input)
	if err != nil {
		return nil, err
	}

	existing.Title = strings.TrimSpace(input.Title)
	existing.Content = input.Content
	existing.Summary = input.Summary
	existing.Status = input.Status
	existing.CoverURL = coverURL
	existing.CoverWidth = coverWidth
	existing.CoverHeight = coverHeight

	post, err := s.saveWithTags(&existing, input.TagIDs)
	if err != nil {
		return nil, err
	}

	return post, nil
}

// Delete removes a post by id.
func (s *PostService) Delete(id uint) error {
	if err := s.db.Delete(&db.Post{}, id).Error; err != nil {
		return err
	}
	return nil
}

// List provides paginated posts with aggregated counters based on filters.
func (s *PostService) List(filter PostFilter) (*PostListResult, error) {
	result := &PostListResult{Page: filter.Page, PerPage: filter.PerPage}
	if result.Page <= 0 {
		result.Page = 1
	}
	if result.PerPage <= 0 {
		result.PerPage = 10
	}

	modelQuery := s.db.Model(&db.Post{})
	modelQuery = s.applyFilters(modelQuery, filter, true)

	if err := modelQuery.Count(&result.Total).Error; err != nil {
		return nil, err
	}

	offset := (result.Page - 1) * result.PerPage

	var posts []db.Post
	dataQuery := s.db.Model(&db.Post{}).
		Preload("Tags").
		Preload("User")
	dataQuery = s.applyFilters(dataQuery, filter, true)

	if err := dataQuery.Order("posts.created_at desc").Limit(result.PerPage).Offset(offset).Find(&posts).Error; err != nil {
		return nil, err
	}

	filterWithoutStatus := filter
	filterWithoutStatus.Status = ""

	baseCounter := s.db.Model(&db.Post{})
	baseCounter = s.applyFilters(baseCounter, filterWithoutStatus, false)

	if err := baseCounter.Where("posts.status = ?", "published").Count(&result.PublishedCount).Error; err != nil {
		return nil, err
	}

	if err := baseCounter.Where("posts.status = ?", "draft").Count(&result.DraftCount).Error; err != nil {
		return nil, err
	}

	if result.Total == 0 {
		result.TotalPages = 1
	} else {
		result.TotalPages = int((result.Total + int64(result.PerPage) - 1) / int64(result.PerPage))
	}

	result.Posts = posts
	return result, nil
}

func (s *PostService) saveWithTags(post *db.Post, tagIDs []uint) (*db.Post, error) {
	return post, s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(post).Error; err != nil {
			return err
		}

		var tags []db.Tag
		if len(tagIDs) > 0 {
			if err := tx.Where("id IN ?", tagIDs).Find(&tags).Error; err != nil {
				return err
			}

			if len(tags) != len(tagIDs) {
				return ErrTagNotFound
			}
		}

		if err := tx.Model(post).Association("Tags").Replace(tags); err != nil {
			return err
		}

		return tx.Preload("Tags").First(post, post.ID).Error
	})
}

func (s *PostService) applyFilters(query *gorm.DB, filter PostFilter, includeStatus bool) *gorm.DB {
	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Where("posts.title LIKE ? OR posts.content LIKE ? OR posts.summary LIKE ?", search, search, search)
	}

	if includeStatus && filter.Status != "" {
		query = query.Where("posts.status = ?", filter.Status)
	}

	if len(filter.TagNames) > 0 {
		subQuery := s.db.Model(&db.Post{}).
			Select("posts.id").
			Joins("JOIN post_tags ON posts.id = post_tags.post_id").
			Joins("JOIN tags ON tags.id = post_tags.tag_id").
			Where("tags.name IN ?", filter.TagNames).
			Distinct()

		query = query.Where("posts.id IN (?)", subQuery)
	}

	if filter.StartDate != nil {
		query = query.Where("posts.created_at >= ?", filter.StartDate)
	}

	if filter.EndDate != nil {
		query = query.Where("posts.created_at <= ?", filter.EndDate)
	}

	return query
}

func normalizeCover(input PostInput) (string, int, int, error) {
	coverURL := strings.TrimSpace(input.CoverURL)
	if coverURL == "" {
		return "", 0, 0, ErrCoverRequired
	}

	if input.CoverWidth <= 0 || input.CoverHeight <= 0 {
		return "", 0, 0, ErrCoverInvalid
	}

	return coverURL, input.CoverWidth, input.CoverHeight, nil
}
