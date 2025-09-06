package db

import "gorm.io/gorm"

// Tag 定义了标签模型
type Tag struct {
	gorm.Model
	Name  string `gorm:"unique;not null"`
	Posts []Post `gorm:"many2many:post_tags;"`
}