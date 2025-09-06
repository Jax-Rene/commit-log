package db

import "gorm.io/gorm"

// User 定义了用户模型
type User struct {
	gorm.Model
	Username string `gorm:"unique;not null"`
	Password string `gorm:"not null"`
}