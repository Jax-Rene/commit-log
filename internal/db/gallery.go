package db

import "gorm.io/gorm"

// GalleryImage 定义摄影作品集图片模型
type GalleryImage struct {
	gorm.Model
	Title       string
	Description string
	ImageURL    string
	ImageWidth  int
	ImageHeight int
	Status      string `gorm:"default:published"` // published, draft
	SortOrder   int    `gorm:"default:0"`
}
