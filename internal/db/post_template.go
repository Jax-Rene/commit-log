package db

import (
	"time"

	"gorm.io/gorm"
)

// PostTemplate 定义文章模板模型。
type PostTemplate struct {
	gorm.Model
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"size:512;default:''"`
	Content     string `gorm:"not null"`
	Summary     string
	Visibility  string `gorm:"size:16;not null;default:public"`
	CoverURL    string
	CoverWidth  int
	CoverHeight int
	UsageCount  int
	LastUsedAt  *time.Time
	Tags        []Tag `gorm:"many2many:post_template_tags;"`
}

// PopulateDerivedFields 填充模板的派生字段。
func (t *PostTemplate) PopulateDerivedFields() {
	t.Visibility = NormalizePostVisibility(t.Visibility)
}

// AfterFind 在查询后规范可见性字段。
func (t *PostTemplate) AfterFind(tx *gorm.DB) error {
	t.PopulateDerivedFields()
	return nil
}
