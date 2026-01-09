package handler

import (
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
)

type analyticsProvider interface {
	Overview(limit int) (service.SiteOverview, error)
	HourlyTrafficTrend(now time.Time, hours int) ([]service.HourlyTrafficPoint, error)
	PostStatsMap(postIDs []uint) (map[uint]*db.PostStatistic, error)
	RecordPostView(postID uint, visitorID string, now time.Time) (*db.PostStatistic, error)
}
