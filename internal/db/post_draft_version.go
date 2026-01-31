package db

import "gorm.io/gorm"

// PostDraftVersion 记录文章草稿的历史版本快照。
type PostDraftVersion struct {
	gorm.Model
	PostID      uint
	Post        Post
	Content     string `gorm:"type:text"`
	Summary     string `gorm:"type:text"`
	ReadingTime int
	CoverURL    string
	CoverWidth  int
	CoverHeight int
	UserID      uint
	User        User
	Version     int
	ContentHash string
	Tags        []Tag `gorm:"many2many:post_draft_version_tags;"`
}

// TableName 指定自定义表名。
func (PostDraftVersion) TableName() string {
	return "post_draft_versions"
}
