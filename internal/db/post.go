package db

import "gorm.io/gorm"

// Post 定义了文章模型
type Post struct {
	gorm.Model
	Title       string
	Content     string
	Summary     string
	ReadingTime int
	UserID      uint
	User        User
	Tags        []Tag `gorm:"many2many:post_tags;"`
}
