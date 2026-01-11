package db

import "gorm.io/gorm"

// Tag 定义了标签模型
type Tag struct {
	gorm.Model
	Name           string            `gorm:"unique;not null"`
	Posts          []Post            `gorm:"many2many:post_tags;"`
	Publications   []PostPublication `gorm:"many2many:post_publication_tags;"`
	Translations   []TagTranslation  `gorm:"constraint:OnDelete:CASCADE;"`
	PostCount      int64             `gorm:"->;column:post_count" json:"post_count"`
	PublishedCount int64             `gorm:"-" json:"published_count"`
}

// TagTranslation stores localized display information for a tag concept.
// Tag.Name remains the stable key (used for filtering and associations).
type TagTranslation struct {
	gorm.Model
	TagID       uint   `gorm:"not null;index:idx_tag_translations_tag_lang,unique"`
	Language    string `gorm:"size:8;not null;index:idx_tag_translations_tag_lang,unique"`
	Name        string `gorm:"not null"`
}
