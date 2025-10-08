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
)

// TagService wraps tag related operations.
type TagService struct {
	db *gorm.DB
}

// NewTagService creates a TagService instance.
func NewTagService(gdb *gorm.DB) *TagService {
	return &TagService{db: gdb}
}

// List returns tags ordered by name.
func (s *TagService) List() ([]db.Tag, error) {
	var tags []db.Tag
	if err := s.db.
		Model(&db.Tag{}).
		Select("tags.*, COUNT(post_tags.post_id) AS post_count").
		Joins("LEFT JOIN post_tags ON post_tags.tag_id = tags.id").
		Group("tags.id").
		Order("tags.name asc").
		Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// ListWithPosts returns tags with their associated posts sorted by creation time.
func (s *TagService) ListWithPosts() ([]db.Tag, error) {
	var tags []db.Tag
	if err := s.db.Preload("Posts").Order("created_at desc").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
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

	tag := db.Tag{Name: name}
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

	return s.db.Delete(&tag).Error
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
