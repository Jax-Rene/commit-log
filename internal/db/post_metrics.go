package db

import "time"

// PostStatistic 汇总文章维度的浏览数据。
type PostStatistic struct {
	ID             uint   `gorm:"primaryKey"`
	PostID         uint   `gorm:"uniqueIndex"`
	PageViews      uint64 `gorm:"default:0"`
	UniqueVisitors uint64 `gorm:"default:0"`
	LastViewedAt   time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TableName 指定自定义表名，避免自动复数化导致的歧义。
func (PostStatistic) TableName() string {
	return "post_statistics"
}

// PostVisit 记录访客层面的浏览历史，用于 UV/PV 去重。
type PostVisit struct {
	ID            uint   `gorm:"primaryKey"`
	PostID        uint   `gorm:"index"`
	VisitorID     string `gorm:"size:64;index"`
	LastViewedAt  time.Time
	LastCountedAt time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TableName 指定自定义表名。
func (PostVisit) TableName() string {
	return "post_visits"
}
