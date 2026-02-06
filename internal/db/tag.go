package db

import "gorm.io/gorm"

// Tag 定义了标签模型
type Tag struct {
	gorm.Model
	Name           string            `gorm:"unique;not null"`
	SortOrder      int               `gorm:"default:0" json:"sort_order"`
	Posts          []Post            `gorm:"many2many:post_tags;"`
	Publications   []PostPublication `gorm:"many2many:post_publication_tags;"`
	PostCount      int64             `gorm:"->;column:post_count" json:"post_count"`
	PublishedCount int64             `gorm:"-" json:"published_count"`
}
