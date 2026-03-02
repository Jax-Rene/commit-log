package service

import (
	"errors"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
)

var (
	ErrTemplateNotFound = errors.New("post template not found")
)

// PostTemplateInput 表示模板创建与更新入参。
type PostTemplateInput struct {
	Name        string
	Description string
	Content     string
	Summary     string
	Visibility  string
	CoverURL    string
	CoverWidth  int
	CoverHeight int
	TagIDs      []uint
}

// TemplateFilter 描述模板列表筛选与分页参数。
type TemplateFilter struct {
	Keyword string
	Page    int
	PerPage int
}

// TemplateListResult 表示模板列表查询结果。
type TemplateListResult struct {
	Templates  []db.PostTemplate
	Total      int64
	TotalPages int
	Page       int
	PerPage    int
}

// TemplateRenderInput 表示模板占位符渲染变量。
type TemplateRenderInput struct {
	Title string
	Now   time.Time
}

// TemplateService 封装模板相关业务逻辑。
type TemplateService struct {
	db *gorm.DB
}

// NewTemplateService 创建模板服务实例。
func NewTemplateService(gdb *gorm.DB) *TemplateService {
	return &TemplateService{db: gdb}
}

// List 返回模板分页列表。
func (s *TemplateService) List(filter TemplateFilter) (*TemplateListResult, error) {
	result := &TemplateListResult{
		Page:    filter.Page,
		PerPage: filter.PerPage,
	}
	if result.Page <= 0 {
		result.Page = 1
	}
	if result.PerPage <= 0 {
		result.PerPage = 20
	}

	query := s.db.Model(&db.PostTemplate{})
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ? OR content LIKE ?", like, like, like)
	}

	if err := query.Count(&result.Total).Error; err != nil {
		return nil, err
	}

	offset := (result.Page - 1) * result.PerPage
	var templates []db.PostTemplate
	if err := query.
		Preload("Tags").
		Order("updated_at desc, id desc").
		Limit(result.PerPage).
		Offset(offset).
		Find(&templates).Error; err != nil {
		return nil, err
	}

	result.Templates = templates
	if result.Total == 0 {
		result.TotalPages = 1
	} else {
		result.TotalPages = int((result.Total + int64(result.PerPage) - 1) / int64(result.PerPage))
	}
	return result, nil
}

// Get 获取单个模板详情。
func (s *TemplateService) Get(id uint) (*db.PostTemplate, error) {
	var template db.PostTemplate
	if err := s.db.Preload("Tags").First(&template, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	return &template, nil
}

// Create 新建模板并写入标签关联。
func (s *TemplateService) Create(input PostTemplateInput) (*db.PostTemplate, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("template name is required")
	}

	visibility, err := normalizeVisibilityInput(input.Visibility, db.PostVisibilityPublic)
	if err != nil {
		return nil, err
	}

	template := &db.PostTemplate{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		Content:     input.Content,
		Summary:     strings.TrimSpace(input.Summary),
		Visibility:  visibility,
		CoverURL:    strings.TrimSpace(input.CoverURL),
		CoverWidth:  input.CoverWidth,
		CoverHeight: input.CoverHeight,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(template).Error; err != nil {
			return err
		}
		return s.replaceTemplateTags(tx, template, input.TagIDs)
	}); err != nil {
		return nil, err
	}

	return template, nil
}

// Update 更新模板基础字段与标签关联。
func (s *TemplateService) Update(id uint, input PostTemplateInput) (*db.PostTemplate, error) {
	var template db.PostTemplate
	if err := s.db.First(&template, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("template name is required")
	}

	visibility, err := normalizeVisibilityInput(input.Visibility, template.Visibility)
	if err != nil {
		return nil, err
	}

	template.Name = name
	template.Description = strings.TrimSpace(input.Description)
	template.Content = input.Content
	template.Summary = strings.TrimSpace(input.Summary)
	template.Visibility = visibility
	template.CoverURL = strings.TrimSpace(input.CoverURL)
	template.CoverWidth = input.CoverWidth
	template.CoverHeight = input.CoverHeight

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&template).Error; err != nil {
			return err
		}
		return s.replaceTemplateTags(tx, &template, input.TagIDs)
	}); err != nil {
		return nil, err
	}

	return s.Get(template.ID)
}

// Delete 物理删除模板，并解除文章来源引用。
func (s *TemplateService) Delete(id uint) error {
	var template db.PostTemplate
	if err := s.db.First(&template, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTemplateNotFound
		}
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&db.Post{}).
			Where("source_template_id = ?", id).
			Update("source_template_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM post_template_tags WHERE post_template_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&template).Error
	})
}

// RenderContent 根据占位符变量渲染模板正文。
func (s *TemplateService) RenderContent(content string, input TemplateRenderInput) string {
	return renderTemplateContent(content, input)
}

func renderTemplateContent(content string, input TemplateRenderInput) string {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	title := strings.TrimSpace(input.Title)

	replacer := strings.NewReplacer(
		"{{date}}", now.Format("2006-01-02"),
		"{{datetime}}", now.Format("2006-01-02 15:04"),
		"{{title}}", title,
	)
	return replacer.Replace(content)
}

func (s *TemplateService) replaceTemplateTags(tx *gorm.DB, template *db.PostTemplate, tagIDs []uint) error {
	var tags []db.Tag
	if len(tagIDs) > 0 {
		if err := tx.Where("id IN ?", tagIDs).Find(&tags).Error; err != nil {
			return err
		}
		if len(tags) != len(tagIDs) {
			return ErrTagNotFound
		}
	}

	if err := tx.Model(template).Association("Tags").Replace(tags); err != nil {
		return err
	}
	return tx.Preload("Tags").First(template, template.ID).Error
}
