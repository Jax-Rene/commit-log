package db

import "gorm.io/gorm"

// ProfileContact 用于保存前台展示的联系与社交信息
// 支持自定义排序、平台与跳转链接
// Icon 字段用于匹配前端内置的图标
// Visible 标记是否在前台 about 页展示
// Sort 值越小越靠前

type ProfileContact struct {
	gorm.Model
	Platform string `gorm:"size:50;not null"`
	Label    string `gorm:"size:80;not null"`
	Value    string `gorm:"size:255;not null"`
	Link     string `gorm:"size:255"`
	Icon     string `gorm:"size:50"`
	Sort     int    `gorm:"default:0"`
	Visible  bool
}

// TableName 返回自定义表名，避免冲突
func (ProfileContact) TableName() string {
	return "profile_contacts"
}
