package service

import (
	"errors"
	"time"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const defaultViewDedupWindow = 30 * time.Minute

// AnalyticsService 负责处理文章浏览相关的统计逻辑。
type AnalyticsService struct {
	db          *gorm.DB
	dedupWindow time.Duration
}

// NewAnalyticsService 创建 AnalyticsService，默认去重窗口为 30 分钟。
func NewAnalyticsService(gdb *gorm.DB) *AnalyticsService {
	return &AnalyticsService{db: gdb, dedupWindow: defaultViewDedupWindow}
}

// WithDedupWindow 允许在测试或特定场景下调整去重窗口。
func (s *AnalyticsService) WithDedupWindow(d time.Duration) *AnalyticsService {
	if d <= 0 {
		return s
	}
	s.dedupWindow = d
	return s
}

// RecordPostView 记录访客对文章的浏览，并返回最新的统计数据。
func (s *AnalyticsService) RecordPostView(postID uint, visitorID string, now time.Time) (*db.PostStatistic, error) {
	if visitorID == "" || postID == 0 {
		return nil, errors.New("invalid visitor or post id")
	}

	var stats db.PostStatistic

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		visit := db.PostVisit{
			PostID:        postID,
			VisitorID:     visitorID,
			LastViewedAt:  now,
			LastCountedAt: now,
		}
		insert := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "post_id"}, {Name: "visitor_id"}},
			DoNothing: true,
		}).Create(&visit)
		if insert.Error != nil {
			return insert.Error
		}

		isNewVisitor := insert.RowsAffected == 1
		if !isNewVisitor {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("post_id = ? AND visitor_id = ?", postID, visitorID).
				First(&visit).Error; err != nil {
				return err
			}
			visit.LastViewedAt = now
			visit.LastCountedAt = now
			if err := tx.Save(&visit).Error; err != nil {
				return err
			}
		}

		statsResult := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("post_id = ?", postID).
			First(&stats)

		switch {
		case errors.Is(statsResult.Error, gorm.ErrRecordNotFound):
			stats = db.PostStatistic{PostID: postID}
			if err := tx.Create(&stats).Error; err != nil {
				return err
			}
		case statsResult.Error != nil:
			return statsResult.Error
		}

		stats.PageViews++
		if isNewVisitor {
			stats.UniqueVisitors++
		}
		stats.LastViewedAt = now

		if err := tx.Save(&stats).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &stats, nil
}

// PostStatsMap 返回指定文章的统计数据，未找到的文章不会出现在结果中。
func (s *AnalyticsService) PostStatsMap(postIDs []uint) (map[uint]*db.PostStatistic, error) {
	result := make(map[uint]*db.PostStatistic, len(postIDs))
	if len(postIDs) == 0 {
		return result, nil
	}

	var stats []db.PostStatistic
	if err := s.db.Where("post_id IN ?", postIDs).Find(&stats).Error; err != nil {
		return nil, err
	}

	for i := range stats {
		stat := stats[i]
		copy := stat
		result[stat.PostID] = &copy
	}

	return result, nil
}

// SiteOverview 聚合站点层面的 UV/PV 数据及热门文章。
type SiteOverview struct {
	TotalPageViews      uint64
	TotalUniqueVisitors uint64
	PostCount           int64
	TopPosts            []TopPostStat
}

// TopPostStat 描述热门文章的统计信息。
type TopPostStat struct {
	PostID         uint
	Title          string
	PageViews      uint64
	UniqueVisitors uint64
}

// Overview 汇总全站 UV/PV。
func (s *AnalyticsService) Overview(limit int) (SiteOverview, error) {
	if limit <= 0 {
		limit = 5
	}

	var overview SiteOverview

	// 总 PV/UV
	var totals struct {
		PageViews      uint64
		UniqueVisitors uint64
	}
	if err := s.db.Model(&db.PostStatistic{}).
		Select("COALESCE(SUM(page_views), 0) AS page_views, COALESCE(SUM(unique_visitors), 0) AS unique_visitors").
		Scan(&totals).Error; err != nil {
		return overview, err
	}
	overview.TotalPageViews = totals.PageViews

	var uniqueVisitors int64
	if err := s.db.Model(&db.PostVisit{}).Distinct("visitor_id").Count(&uniqueVisitors).Error; err != nil {
		return overview, err
	}
	overview.TotalUniqueVisitors = uint64(uniqueVisitors)

	if err := s.db.Model(&db.Post{}).Count(&overview.PostCount).Error; err != nil {
		return overview, err
	}

	var topPosts []TopPostStat
	if err := s.db.Table("post_statistics ps").
		Select("ps.post_id, p.title, ps.page_views, ps.unique_visitors").
		Joins("JOIN posts p ON p.id = ps.post_id").
		Order("ps.page_views DESC").
		Limit(limit).
		Scan(&topPosts).Error; err != nil {
		return overview, err
	}

	overview.TopPosts = topPosts
	return overview, nil
}
