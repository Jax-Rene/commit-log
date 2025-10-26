package db

import (
	"time"

	"gorm.io/gorm"
)

// Post 定义了文章模型
type Post struct {
	gorm.Model
	Title       string
	Content     string
	Summary     string
	Status      string `gorm:"default:draft"` // draft, published
	ReadingTime int
	CoverURL    string
	CoverWidth  int
	CoverHeight int
	UserID      uint
	User        User
	Tags        []Tag `gorm:"many2many:post_tags;"`
	PublishedAt time.Time
	// PublicationCount 记录文章发布次数，用于版本号展示
	PublicationCount int
	// LatestPublicationID 指向最近一次发布的快照
	LatestPublicationID *uint
}

// PostPublication 存储文章发布时的快照数据
type PostPublication struct {
	gorm.Model
	PostID      uint
	Post        Post
	Title       string
	Content     string
	Summary     string
	ReadingTime int
	CoverURL    string
	CoverWidth  int
	CoverHeight int
	UserID      uint
	User        User
	PublishedAt time.Time
	Version     int
	Tags        []Tag `gorm:"many2many:post_publication_tags;"`
}
