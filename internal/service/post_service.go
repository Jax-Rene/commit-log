package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
)

var (
	ErrPostNotFound        = errors.New("post not found")
	ErrCoverRequired       = errors.New("cover image is required")
	ErrCoverInvalid        = errors.New("cover dimensions are invalid")
	ErrPublicationNotFound = errors.New("post publication not found")
	ErrInvalidPublishState = errors.New("post is missing required fields for publishing")
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

// PublicationListResult 用于前台展示发布快照
type PublicationListResult struct {
	Publications []db.PostPublication
	Total        int64
	TotalPages   int
	Page         int
	PerPage      int
}

// PostInput represents fields accepted when creating or updating a post.
type PostInput struct {
	Title       string
	Content     string
	Summary     string
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
	for i := range posts {
		posts[i].PopulateDerivedFields()
	}
	return posts, nil
}

// Get fetches a post by id with tags preloaded.
func (s *PostService) Get(id uint) (*db.Post, error) {
	var post db.Post
	if err := s.db.Preload("Tags").Preload("User").First(&post, id).Error; err != nil {
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
		Content:     input.Content,
		Summary:     strings.TrimSpace(input.Summary),
		Status:      "draft",
		UserID:      input.UserID,
		CoverURL:    coverURL,
		CoverWidth:  coverWidth,
		CoverHeight: coverHeight,
		ReadingTime: calculateReadingTime(input.Content),
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

	existing.Content = input.Content
	existing.Summary = strings.TrimSpace(input.Summary)
	existing.CoverURL = coverURL
	existing.CoverWidth = coverWidth
	existing.CoverHeight = coverHeight
	existing.ReadingTime = calculateReadingTime(input.Content)

	post, err := s.saveWithTags(&existing, input.TagIDs)
	if err != nil {
		return nil, err
	}

	return post, nil
}

// UpdateSummary 仅更新文章摘要字段。
func (s *PostService) UpdateSummary(id uint, summary string) error {
	trimmed := strings.TrimSpace(summary)
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&db.Post{}).Where("id = ?", id).Update("summary", trimmed).Error; err != nil {
			return err
		}

		if trimmed == "" {
			return nil
		}

		var publication db.PostPublication
		if err := tx.Where("post_id = ?", id).
			Order("published_at desc, id desc").
			First(&publication).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		return tx.Model(&db.PostPublication{}).
			Where("id = ?", publication.ID).
			Update("summary", trimmed).Error
	})
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

	orderBy := "posts.created_at desc"
	if strings.EqualFold(filter.Status, "published") {
		orderBy = "posts.published_at desc, posts.id desc"
	}

	if err := dataQuery.Order(orderBy).Limit(result.PerPage).Offset(offset).Find(&posts).Error; err != nil {
		return nil, err
	}

	for i := range posts {
		posts[i].PopulateDerivedFields()
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

// Publish 创建文章发布快照，并更新文章发布状态
func (s *PostService) Publish(postID, userID uint, publishedAt *time.Time) (*db.PostPublication, error) {
	var post db.Post
	if err := s.db.Preload("Tags").First(&post, postID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	post.PopulateDerivedFields()

	if strings.TrimSpace(post.Title) == "" {
		return nil, ErrInvalidPublishState
	}
	if strings.TrimSpace(post.Content) == "" {
		return nil, ErrInvalidPublishState
	}
	if strings.TrimSpace(post.CoverURL) == "" {
		return nil, ErrCoverRequired
	}
	if post.CoverWidth <= 0 || post.CoverHeight <= 0 {
		return nil, ErrCoverInvalid
	}

	readingTime := calculateReadingTime(post.Content)
	version := post.PublicationCount + 1

	publishTime := time.Now()
	if publishedAt != nil && !publishedAt.IsZero() {
		publishTime = *publishedAt
	}

	publication := db.PostPublication{
		PostID:      post.ID,
		Content:     post.Content,
		Summary:     post.Summary,
		ReadingTime: readingTime,
		CoverURL:    post.CoverURL,
		CoverWidth:  post.CoverWidth,
		CoverHeight: post.CoverHeight,
		UserID:      userID,
		PublishedAt: publishTime,
		Version:     version,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&publication).Error; err != nil {
			return err
		}

		if len(post.Tags) > 0 {
			if err := tx.Model(&publication).Association("Tags").Replace(post.Tags); err != nil {
				return err
			}
		}

		updates := map[string]interface{}{
			"status":                "published",
			"reading_time":          readingTime,
			"published_at":          publishTime,
			"publication_count":     version,
			"latest_publication_id": publication.ID,
		}

		if err := tx.Model(&db.Post{}).
			Where("id = ?", post.ID).
			Updates(updates).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if err := s.db.Preload("Tags").Preload("User").First(&publication, publication.ID).Error; err != nil {
		return nil, err
	}

	publication.PopulateDerivedFields()
	return &publication, nil
}

// LatestPublication 返回文章最近一次发布快照
func (s *PostService) LatestPublication(postID uint) (*db.PostPublication, error) {
	var publication db.PostPublication
	if err := s.db.Preload("Tags").
		Preload("User").
		Joins("JOIN posts ON posts.latest_publication_id = post_publications.id").
		Where("posts.id = ?", postID).
		First(&publication).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPublicationNotFound
		}
		return nil, err
	}
	publication.PopulateDerivedFields()
	return &publication, nil
}

// ListPublished 返回最新发布的文章快照列表
func (s *PostService) ListPublished(filter PostFilter) (*PublicationListResult, error) {
	result := &PublicationListResult{Page: filter.Page, PerPage: filter.PerPage}
	if result.Page <= 0 {
		result.Page = 1
	}
	if result.PerPage <= 0 {
		result.PerPage = 10
	}

	baseQuery := s.db.Model(&db.PostPublication{}).
		Joins("JOIN posts ON posts.latest_publication_id = post_publications.id").
		Where("posts.status = ?", "published")
	baseQuery = s.applyPublicationFilters(baseQuery, filter)

	if err := baseQuery.Count(&result.Total).Error; err != nil {
		return nil, err
	}

	offset := (result.Page - 1) * result.PerPage

	var publications []db.PostPublication
	dataQuery := s.db.Model(&db.PostPublication{}).
		Preload("Tags").
		Preload("User").
		Joins("JOIN posts ON posts.latest_publication_id = post_publications.id").
		Where("posts.status = ?", "published")
	dataQuery = s.applyPublicationFilters(dataQuery, filter)

	if err := dataQuery.
		Order("post_publications.published_at desc, post_publications.id desc").
		Limit(result.PerPage).
		Offset(offset).
		Find(&publications).Error; err != nil {
		return nil, err
	}

	for i := range publications {
		publications[i].PopulateDerivedFields()
	}

	if result.Total == 0 {
		result.TotalPages = 1
	} else {
		result.TotalPages = int((result.Total + int64(result.PerPage) - 1) / int64(result.PerPage))
	}

	result.Publications = publications
	return result, nil
}

// ListAllPublished 返回所有文章的最新发布快照
func (s *PostService) ListAllPublished() ([]db.PostPublication, error) {
	var publications []db.PostPublication
	if err := s.db.Preload("Tags").
		Joins("JOIN posts ON posts.latest_publication_id = post_publications.id").
		Where("posts.status = ?", "published").
		Order("post_publications.published_at desc, post_publications.id desc").
		Find(&publications).Error; err != nil {
		return nil, err
	}
	for i := range publications {
		publications[i].PopulateDerivedFields()
	}
	return publications, nil
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

		if err := tx.Preload("Tags").First(post, post.ID).Error; err != nil {
			return err
		}

		post.PopulateDerivedFields()
		return nil
	})
}

func (s *PostService) applyFilters(query *gorm.DB, filter PostFilter, includeStatus bool) *gorm.DB {
	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		alias := "posts"
		titleExpr := derivedTitleQueryExpr(alias)
		query = query.Where(fmt.Sprintf("(%s LIKE ? OR %s.content LIKE ? OR %s.summary LIKE ?)", titleExpr, alias, alias), search, search, search)
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

func (s *PostService) applyPublicationFilters(query *gorm.DB, filter PostFilter) *gorm.DB {
	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		alias := "post_publications"
		titleExpr := derivedTitleQueryExpr(alias)
		query = query.Where(fmt.Sprintf("(%s LIKE ? OR %s.content LIKE ? OR %s.summary LIKE ?)", titleExpr, alias, alias), search, search, search)
	}

	if len(filter.TagNames) > 0 {
		subQuery := s.db.Model(&db.PostPublication{}).
			Select("post_publications.id").
			Joins("JOIN post_publication_tags ON post_publication_tags.post_publication_id = post_publications.id").
			Joins("JOIN tags ON tags.id = post_publication_tags.tag_id").
			Where("tags.name IN ?", filter.TagNames)

		query = query.Where("post_publications.id IN (?)", subQuery)
	}

	if filter.StartDate != nil {
		query = query.Where("post_publications.published_at >= ?", filter.StartDate)
	}

	if filter.EndDate != nil {
		query = query.Where("post_publications.published_at <= ?", filter.EndDate)
	}

	return query
}

func normalizeCover(input PostInput) (string, int, int, error) {
	coverURL := strings.TrimSpace(input.CoverURL)
	if coverURL == "" {
		return "", 0, 0, nil
	}

	if input.CoverWidth <= 0 || input.CoverHeight <= 0 {
		return "", 0, 0, ErrCoverInvalid
	}

	return coverURL, input.CoverWidth, input.CoverHeight, nil
}

func calculateReadingTime(content string) int {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return 0
	}

	runes := []rune(trimmed)
	if len(runes) == 0 {
		return 0
	}

	minutes := len(runes) / 400
	if len(runes)%400 != 0 {
		minutes++
	}
	if minutes < 1 {
		minutes = 1
	}
	return minutes
}

func derivedTitleQueryExpr(alias string) string {
	line := fmt.Sprintf("CASE WHEN instr(%s.content, char(10)) > 0 THEN substr(%s.content, 1, instr(%s.content, char(10)) - 1) ELSE %s.content END", alias, alias, alias, alias)
	trimmed := fmt.Sprintf("TRIM(RTRIM(LTRIM(TRIM(%s), '#'), '#'))", line)
	return trimmed
}
