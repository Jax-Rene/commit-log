package service

import (
	"errors"
	"strings"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
)

var (
	ErrTagExists   = errors.New("tag already exists")
	ErrTagInUse    = errors.New("tag is associated with posts")
	ErrTagNotFound = errors.New("tag not found")
	ErrTagOrder    = errors.New("invalid tag order")
)

// TagService wraps tag related operations.
type TagService struct {
	db *gorm.DB
}

// TagUsage 描述标签的使用次数
type TagUsage struct {
	ID    uint
	Name  string
	Count int64
}

// NewTagService creates a TagService instance.
func NewTagService(gdb *gorm.DB) *TagService {
	return &TagService{db: gdb}
}

// List returns tags ordered by configured sort order.
func (s *TagService) List() ([]db.Tag, error) {
	var tags []db.Tag
	if err := s.db.
		Model(&db.Tag{}).
		Select("tags.*, COUNT(post_tags.post_id) AS post_count").
		Joins("LEFT JOIN post_tags ON post_tags.tag_id = tags.id").
		Group("tags.id").
		Order("tags.sort_order asc").
		Order("tags.name asc").
		Order("tags.id asc").
		Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// ListWithPosts returns tags with their associated posts in configured order.
func (s *TagService) ListWithPosts() ([]db.Tag, error) {
	var tags []db.Tag
	if err := s.db.Preload("Posts").Order("sort_order asc").Order("name asc").Order("id asc").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// PublishedUsage 返回已发布文章中标签的使用统计
func (s *TagService) PublishedUsage() ([]TagUsage, error) {
	var rows []struct {
		ID    uint
		Name  string
		Count int64
	}

	query := s.db.Table("tags").
		Select("tags.id, tags.name, COUNT(DISTINCT post_publications.id) AS count").
		Joins("JOIN post_publication_tags ON post_publication_tags.tag_id = tags.id").
		Joins("JOIN post_publications ON post_publications.id = post_publication_tags.post_publication_id").
		Joins("JOIN posts ON posts.latest_publication_id = post_publications.id").
		Where("posts.status = ?", "published").
		Group("tags.id, tags.name").
		Order("tags.sort_order asc").
		Order("tags.name asc").
		Order("tags.id asc")

	if err := query.Scan(&rows).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []TagUsage{}, nil
		}
		return nil, err
	}

	usages := make([]TagUsage, 0, len(rows))
	for _, row := range rows {
		usages = append(usages, TagUsage{
			ID:    row.ID,
			Name:  row.Name,
			Count: row.Count,
		})
	}

	return usages, nil
}

// Create inserts a new tag with unique name.
func (s *TagService) Create(name string) (*db.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("tag name is required")
	}

	var existing db.Tag
	if err := s.db.Where("name = ?", name).First(&existing).Error; err == nil {
		return nil, ErrTagExists
	}

	sortOrder, err := s.nextSortOrder()
	if err != nil {
		return nil, err
	}

	tag := db.Tag{Name: name, SortOrder: sortOrder}
	if err := s.db.Create(&tag).Error; err != nil {
		return nil, err
	}
	tag.PostCount = 0

	return &tag, nil
}

// Update changes the tag name while keeping uniqueness.
func (s *TagService) Update(id uint, name string) (*db.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("tag name is required")
	}

	var tag db.Tag
	if err := s.db.First(&tag, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTagNotFound
		}
		return nil, err
	}

	var existing db.Tag
	if err := s.db.Where("name = ? AND id <> ?", name, id).First(&existing).Error; err == nil {
		return nil, ErrTagExists
	}

	tag.Name = name
	if err := s.db.Save(&tag).Error; err != nil {
		return nil, err
	}

	count, err := s.postUsageCount(tag.ID)
	if err != nil {
		return nil, err
	}
	tag.PostCount = count

	return &tag, nil
}

// Delete removes a tag if it is not associated with posts.
func (s *TagService) Delete(id uint) error {
	var tag db.Tag
	if err := s.db.First(&tag, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTagNotFound
		}
		return err
	}

	count, err := s.postUsageCount(id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrTagInUse
	}

	return s.db.Unscoped().Delete(&tag).Error
}

// Reorder updates tag sort order based on the provided ids sequence.
func (s *TagService) Reorder(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return ErrTagOrder
		}
		if _, ok := seen[id]; ok {
			return ErrTagOrder
		}
		seen[id] = struct{}{}
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for idx, id := range ids {
			result := tx.Model(&db.Tag{}).Where("id = ?", id).Update("sort_order", idx)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrTagNotFound
			}
		}
		return nil
	})
}

func (s *TagService) postUsageCount(id uint) (int64, error) {
	var count int64
	if err := s.db.Model(&db.Post{}).
		Joins("JOIN post_tags ON posts.id = post_tags.post_id").
		Where("post_tags.tag_id = ?", id).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *TagService) nextSortOrder() (int, error) {
	var maxSort int
	if err := s.db.Model(&db.Tag{}).Select("COALESCE(MAX(sort_order), -1)").Scan(&maxSort).Error; err != nil {
		return 0, err
	}
	return maxSort + 1, nil
}
