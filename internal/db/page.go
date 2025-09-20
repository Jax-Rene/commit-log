package db

import "gorm.io/gorm"

// Page represents a standalone content page such as About.
type Page struct {
	gorm.Model
	Slug    string `gorm:"uniqueIndex;not null"`
	Title   string `gorm:"not null"`
	Summary string
	Content string `gorm:"type:text"`
}
