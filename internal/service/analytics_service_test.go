package service

import (
	"testing"
	"time"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAnalyticsTestDB(t *testing.T) func() {
	t.Helper()

	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := gdb.AutoMigrate(&db.Post{}, &db.PostStatistic{}, &db.PostVisit{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	db.DB = gdb

	return func() {
		sqlDB, err := db.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestRecordPostViewCounts(t *testing.T) {
	cleanup := setupAnalyticsTestDB(t)
	defer cleanup()

	post := db.Post{Title: "测试文章", Status: "published"}
	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to create post: %v", err)
	}

	svc := NewAnalyticsService(db.DB).WithDedupWindow(time.Minute)
	base := time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)

	stats, err := svc.RecordPostView(post.ID, "visitor-1", base)
	if err != nil {
		t.Fatalf("first view failed: %v", err)
	}

	if stats.PageViews != 1 || stats.UniqueVisitors != 1 {
		t.Fatalf("expected PV=1 UV=1, got PV=%d UV=%d", stats.PageViews, stats.UniqueVisitors)
	}

	stats, err = svc.RecordPostView(post.ID, "visitor-1", base.Add(30*time.Second))
	if err != nil {
		t.Fatalf("second quick view failed: %v", err)
	}

	if stats.PageViews != 2 || stats.UniqueVisitors != 1 {
		t.Fatalf("expected PV=2 UV=1 after quick revisit, got PV=%d UV=%d", stats.PageViews, stats.UniqueVisitors)
	}

	stats, err = svc.RecordPostView(post.ID, "visitor-1", base.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("third view failed: %v", err)
	}

	if stats.PageViews != 3 || stats.UniqueVisitors != 1 {
		t.Fatalf("expected PV=3 UV=1 after third view, got PV=%d UV=%d", stats.PageViews, stats.UniqueVisitors)
	}

	stats, err = svc.RecordPostView(post.ID, "visitor-2", base.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("second visitor failed: %v", err)
	}

	if stats.PageViews != 4 || stats.UniqueVisitors != 2 {
		t.Fatalf("expected PV=4 UV=2, got PV=%d UV=%d", stats.PageViews, stats.UniqueVisitors)
	}

	var visit db.PostVisit
	if err := db.DB.Where("post_id = ? AND visitor_id = ?", post.ID, "visitor-1").First(&visit).Error; err != nil {
		t.Fatalf("failed to load visit record: %v", err)
	}

	if !visit.LastViewedAt.Equal(base.Add(2 * time.Minute)) {
		t.Fatalf("unexpected LastViewedAt: %v", visit.LastViewedAt)
	}

	if !visit.LastCountedAt.Equal(base.Add(2 * time.Minute)) {
		t.Fatalf("unexpected LastCountedAt: %v", visit.LastCountedAt)
	}
}

func TestPostStatsMap(t *testing.T) {
	cleanup := setupAnalyticsTestDB(t)
	defer cleanup()

	posts := []db.Post{{Title: "A", Status: "published"}, {Title: "B", Status: "published"}}
	if err := db.DB.Create(&posts).Error; err != nil {
		t.Fatalf("failed to create posts: %v", err)
	}

	svc := NewAnalyticsService(db.DB).WithDedupWindow(time.Second)
	base := time.Now().UTC()

	if _, err := svc.RecordPostView(posts[0].ID, "v1", base); err != nil {
		t.Fatalf("record view failed: %v", err)
	}
	if _, err := svc.RecordPostView(posts[1].ID, "v1", base); err != nil {
		t.Fatalf("record view failed: %v", err)
	}
	if _, err := svc.RecordPostView(posts[1].ID, "v2", base.Add(2*time.Second)); err != nil {
		t.Fatalf("record view failed: %v", err)
	}

	statsMap, err := svc.PostStatsMap([]uint{posts[0].ID, posts[1].ID})
	if err != nil {
		t.Fatalf("PostStatsMap returned error: %v", err)
	}

	if len(statsMap) != 2 {
		t.Fatalf("expected stats map size 2, got %d", len(statsMap))
	}

	if stat := statsMap[posts[0].ID]; stat == nil || stat.PageViews != 1 || stat.UniqueVisitors != 1 {
		t.Fatalf("unexpected stats for post A: %+v", stat)
	}

	if stat := statsMap[posts[1].ID]; stat == nil || stat.PageViews != 2 || stat.UniqueVisitors != 2 {
		t.Fatalf("unexpected stats for post B: %+v", stat)
	}
}

func TestOverview(t *testing.T) {
	cleanup := setupAnalyticsTestDB(t)
	defer cleanup()

	posts := []db.Post{{Title: "One", Status: "published"}, {Title: "Two", Status: "published"}, {Title: "Three", Status: "published"}}
	if err := db.DB.Create(&posts).Error; err != nil {
		t.Fatalf("failed to create posts: %v", err)
	}

	svc := NewAnalyticsService(db.DB).WithDedupWindow(time.Second)
	base := time.Date(2024, 6, 1, 8, 0, 0, 0, time.UTC)

	if _, err := svc.RecordPostView(posts[0].ID, "v1", base); err != nil {
		t.Fatalf("record view p1v1 failed: %v", err)
	}
	if _, err := svc.RecordPostView(posts[0].ID, "v2", base.Add(time.Second)); err != nil {
		t.Fatalf("record view p1v2 failed: %v", err)
	}
	if _, err := svc.RecordPostView(posts[1].ID, "v1", base.Add(2*time.Second)); err != nil {
		t.Fatalf("record view p2v1 failed: %v", err)
	}
	if _, err := svc.RecordPostView(posts[1].ID, "v3", base.Add(3*time.Second)); err != nil {
		t.Fatalf("record view p2v3 failed: %v", err)
	}
	if _, err := svc.RecordPostView(posts[1].ID, "v1", base.Add(4*time.Second)); err != nil {
		t.Fatalf("record view p2v1 second failed: %v", err)
	}
	if _, err := svc.RecordPostView(posts[2].ID, "v4", base.Add(5*time.Second)); err != nil {
		t.Fatalf("record view p3v4 failed: %v", err)
	}

	overview, err := svc.Overview(2)
	if err != nil {
		t.Fatalf("Overview returned error: %v", err)
	}

	if overview.TotalPageViews != 6 {
		t.Fatalf("expected total PV 6, got %d", overview.TotalPageViews)
	}

	if overview.TotalUniqueVisitors != 4 {
		t.Fatalf("expected total UV 4, got %d", overview.TotalUniqueVisitors)
	}

	if overview.PostCount != 3 {
		t.Fatalf("expected post count 3, got %d", overview.PostCount)
	}

	if len(overview.TopPosts) != 2 {
		t.Fatalf("expected top posts size 2, got %d", len(overview.TopPosts))
	}

	if overview.TopPosts[0].PageViews < overview.TopPosts[1].PageViews {
		t.Fatal("expected top posts ordered by PV desc")
	}
}
