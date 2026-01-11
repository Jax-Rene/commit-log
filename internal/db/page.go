package db

import "gorm.io/gorm"

// Page represents a standalone content page such as About.
type Page struct {
	gorm.Model
	Slug     string `gorm:"index:idx_pages_slug_lang,unique;not null"`
	Language string `gorm:"size:8;index:idx_pages_slug_lang,unique;default:zh"`
	Title    string `gorm:"not null"`
	Summary  string
	Content  string `gorm:"type:text"`
}
