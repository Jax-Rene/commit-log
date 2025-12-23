package db

import "time"

// SiteHourlySnapshot 记录站点每小时的 PV/UV 快照。
type SiteHourlySnapshot struct {
	ID             uint      `gorm:"primaryKey"`
	Hour           time.Time `gorm:"uniqueIndex"`
	PageViews      uint64    `gorm:"default:0"`
	UniqueVisitors uint64    `gorm:"default:0"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TableName 指定自定义表名。
func (SiteHourlySnapshot) TableName() string {
	return "site_hourly_snapshots"
}

// SiteHourlyVisitor 记录每小时的访客，用于 UV 去重。
type SiteHourlyVisitor struct {
	ID        uint      `gorm:"primaryKey"`
	Hour      time.Time `gorm:"uniqueIndex:idx_site_hour_visitor"`
	VisitorID string    `gorm:"size:64;uniqueIndex:idx_site_hour_visitor"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName 指定自定义表名。
func (SiteHourlyVisitor) TableName() string {
	return "site_hourly_visitors"
}
