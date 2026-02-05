package service

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
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
	linkPattern            = regexp.MustCompile(`\[[^\]]+\]\([^\)]+\)`)
	imagePattern           = regexp.MustCompile(`!\[[^\]]*\]\([^\)]+\)`)
	bareURLPattern         = regexp.MustCompile(`https?://\S+`)
)

const maxDraftVersions = 10

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
	Sort      string
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
	Title          string
	Content        string
	Summary        string
	TagIDs         []uint
	UserID         uint
	CoverURL       string
	CoverWidth     int
	CoverHeight    int
	DraftSessionID string
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

	return s.saveWithTags(&post, input.TagIDs, input.UserID, input.DraftSessionID)
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

	post, err := s.saveWithTags(&existing, input.TagIDs, input.UserID, input.DraftSessionID)
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

	orderBy := "posts.created_at desc, posts.id desc"
	if strings.EqualFold(strings.TrimSpace(filter.Sort), "hot") {
		dataQuery = dataQuery.Joins("LEFT JOIN post_statistics ps ON ps.post_id = posts.id")
		orderBy = "COALESCE(ps.page_views, 0) desc, COALESCE(ps.unique_visitors, 0) desc, posts.created_at desc, posts.id desc"
	}

	if err := dataQuery.Order(orderBy).Limit(result.PerPage).Offset(offset).Find(&posts).Error; err != nil {
		return nil, err
	}

	for i := range posts {
		posts[i].PopulateDerivedFields()
	}

	filterWithoutStatus := filter
	filterWithoutStatus.Status = ""

	counterBuilder := func() *gorm.DB {
		base := s.db.Model(&db.Post{})
		return s.applyFilters(base, filterWithoutStatus, false)
	}

	// 独立查询已发布与草稿数量，避免状态条件被重复叠加
	if err := counterBuilder().Where("posts.status = ?", "published").Count(&result.PublishedCount).Error; err != nil {
		return nil, err
	}

	if err := counterBuilder().Where("posts.status = ?", "draft").Count(&result.DraftCount).Error; err != nil {
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

// LatestDraft 返回指定用户最近编辑的草稿。
func (s *PostService) LatestDraft(userID uint) (*db.Post, error) {
	query := s.db.Preload("Tags").Preload("User").Where("status = ?", "draft")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	var post db.Post
	if err := query.Order("updated_at desc, id desc").First(&post).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	post.PopulateDerivedFields()
	return &post, nil
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

// ListDraftVersions 返回指定文章的草稿历史版本。
func (s *PostService) ListDraftVersions(postID uint, limit int) ([]db.PostDraftVersion, error) {
	if postID == 0 {
		return nil, ErrPostNotFound
	}

	if err := s.db.Select("id").First(&db.Post{}, postID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	if limit <= 0 || limit > maxDraftVersions {
		limit = maxDraftVersions
	}

	var versions []db.PostDraftVersion
	if err := s.db.Preload("Tags").
		Preload("User").
		Where("post_id = ?", postID).
		Order("created_at desc, id desc").
		Limit(limit).
		Find(&versions).Error; err != nil {
		return nil, err
	}

	return versions, nil
}

func (s *PostService) saveWithTags(post *db.Post, tagIDs []uint, userID uint, draftSessionID string) (*db.Post, error) {
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
		if err := s.recordDraftVersion(tx, post, userID, draftSessionID); err != nil {
			return err
		}
		return nil
	})
}

func (s *PostService) recordDraftVersion(tx *gorm.DB, post *db.Post, userID uint, draftSessionID string) error {
	if tx == nil || post == nil {
		return errors.New("draft version requires valid post and transaction")
	}

	contentHash := hashDraftContent(post.Content)
	sessionID := strings.TrimSpace(draftSessionID)

	if sessionID != "" {
		var latest db.PostDraftVersion
		if err := tx.Where("post_id = ?", post.ID).
			Order("created_at desc, id desc").
			First(&latest).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else if latest.SessionID == sessionID {
			return s.updateDraftVersion(tx, &latest, post, userID, contentHash, sessionID)
		}
	} else {
		var latest db.PostDraftVersion
		if err := tx.Select("id", "content_hash").
			Where("post_id = ?", post.ID).
			Order("created_at desc, id desc").
			First(&latest).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else if latest.ContentHash == contentHash {
			return nil
		}
	}

	resolvedUserID := userID
	if resolvedUserID == 0 {
		resolvedUserID = post.UserID
	}

	var maxVersion int64
	if err := tx.Model(&db.PostDraftVersion{}).
		Where("post_id = ?", post.ID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error; err != nil {
		return err
	}

	draft := db.PostDraftVersion{
		PostID:      post.ID,
		Content:     post.Content,
		ContentHash: contentHash,
		SessionID:   sessionID,
		Summary:     post.Summary,
		ReadingTime: calculateReadingTime(post.Content),
		CoverURL:    post.CoverURL,
		CoverWidth:  post.CoverWidth,
		CoverHeight: post.CoverHeight,
		UserID:      resolvedUserID,
		Version:     int(maxVersion) + 1,
	}

	if err := tx.Create(&draft).Error; err != nil {
		return err
	}

	if len(post.Tags) > 0 {
		if err := tx.Model(&draft).Association("Tags").Replace(post.Tags); err != nil {
			return err
		}
	}

	var staleIDs []uint
	if err := tx.Model(&db.PostDraftVersion{}).
		Where("post_id = ?", post.ID).
		Order("created_at desc, id desc").
		Offset(maxDraftVersions).
		Pluck("id", &staleIDs).Error; err != nil {
		return err
	}

	if len(staleIDs) == 0 {
		return nil
	}

	if err := tx.Where("id IN ?", staleIDs).Delete(&db.PostDraftVersion{}).Error; err != nil {
		return err
	}

	if err := tx.Exec("DELETE FROM post_draft_version_tags WHERE post_draft_version_id IN ?", staleIDs).Error; err != nil {
		return err
	}

	return nil
}

func (s *PostService) updateDraftVersion(tx *gorm.DB, draft *db.PostDraftVersion, post *db.Post, userID uint, contentHash, sessionID string) error {
	if draft == nil {
		return errors.New("draft version requires valid draft")
	}

	resolvedUserID := userID
	if resolvedUserID == 0 {
		resolvedUserID = post.UserID
	}

	updates := map[string]interface{}{
		"content":      post.Content,
		"content_hash": contentHash,
		"session_id":   sessionID,
		"summary":      post.Summary,
		"reading_time": calculateReadingTime(post.Content),
		"cover_url":    post.CoverURL,
		"cover_width":  post.CoverWidth,
		"cover_height": post.CoverHeight,
		"user_id":      resolvedUserID,
	}

	if err := tx.Model(draft).Updates(updates).Error; err != nil {
		return err
	}

	if err := tx.Model(draft).Association("Tags").Replace(post.Tags); err != nil {
		return err
	}

	return nil
}

func hashDraftContent(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

func (s *PostService) applyFilters(query *gorm.DB, filter PostFilter, includeStatus bool) *gorm.DB {
	if tokens := splitSearchTokens(filter.Search); len(tokens) > 0 {
		alias := "posts"
		titleExpr := derivedTitleQueryExpr(alias)
		for _, token := range tokens {
			search := "%" + token + "%"
			query = query.Where(fmt.Sprintf("(%s LIKE ? OR %s.content LIKE ? OR %s.summary LIKE ?)", titleExpr, alias, alias), search, search, search)
		}
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
	if tokens := splitSearchTokens(filter.Search); len(tokens) > 0 {
		alias := "post_publications"
		titleExpr := derivedTitleQueryExpr(alias)
		for _, token := range tokens {
			search := "%" + token + "%"
			query = query.Where(fmt.Sprintf("(%s LIKE ? OR %s.content LIKE ? OR %s.summary LIKE ?)", titleExpr, alias, alias), search, search, search)
		}
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

	sanitized := imagePattern.ReplaceAllString(trimmed, "")
	sanitized = linkPattern.ReplaceAllStringFunc(sanitized, func(match string) string {
		open := strings.Index(match, "[")
		close := strings.Index(match, "]")
		if open >= 0 && close > open {
			return match[open+1 : close]
		}
		return ""
	})
	sanitized = bareURLPattern.ReplaceAllString(sanitized, "")
	sanitized = strings.TrimSpace(sanitized)
	if sanitized == "" {
		return 0
	}

	runes := []rune(sanitized)
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

// CalculateReadingTime exposes the reading time estimator for other packages.
func CalculateReadingTime(content string) int {
	return calculateReadingTime(content)
}

func derivedTitleQueryExpr(alias string) string {
	line := fmt.Sprintf("CASE WHEN instr(%s.content, char(10)) > 0 THEN substr(%s.content, 1, instr(%s.content, char(10)) - 1) ELSE %s.content END", alias, alias, alias, alias)
	trimmed := fmt.Sprintf("TRIM(RTRIM(LTRIM(TRIM(%s), '#'), '#'))", line)
	return trimmed
}

func splitSearchTokens(search string) []string {
	return strings.Fields(search)
}
