package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type stubHTMLRender struct {
	lastName string
	lastData interface{}
}

type stubHTMLInstance struct {
	name string
	data interface{}
}

func (r *stubHTMLRender) Instance(name string, data interface{}) render.Render {
	r.lastName = name
	r.lastData = data
	return &stubHTMLInstance{name: name, data: data}
}

func (r *stubHTMLInstance) Render(http.ResponseWriter) error {
	return nil
}

func (r *stubHTMLInstance) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

type analyticsStub struct {
	trendCalled  bool
	trendHours   int
	overviewUsed int
}

func (a *analyticsStub) Overview(limit int) (service.SiteOverview, error) {
	a.overviewUsed = limit
	return service.SiteOverview{}, nil
}

func (a *analyticsStub) HourlyTrafficTrend(_ time.Time, hours int) ([]service.HourlyTrafficPoint, error) {
	a.trendCalled = true
	a.trendHours = hours
	return nil, nil
}

func (a *analyticsStub) PostStatsMap([]uint) (map[uint]*db.PostStatistic, error) {
	return map[uint]*db.PostStatistic{}, nil
}

func (a *analyticsStub) RecordPostView(uint, string, time.Time) (*db.PostStatistic, error) {
	return &db.PostStatistic{}, nil
}

func setupAdminHandlerTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	dsn := fmt.Sprintf("file:admin-handler-%d?mode=memory&cache=shared", time.Now().UnixNano())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := gdb.AutoMigrate(&db.User{}, &db.Post{}, &db.PostPublication{}, &db.Tag{}, &db.SystemSetting{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return gdb, func() {
		sqlDB, err := gdb.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestShowDashboardUsesSevenDayTrend(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gdb, cleanup := setupAdminHandlerTestDB(t)
	t.Cleanup(cleanup)

	stubAnalytics := &analyticsStub{}
	api := &API{
		db:        gdb,
		posts:     service.NewPostService(gdb),
		system:    service.NewSystemSettingService(gdb),
		analytics: stubAnalytics,
	}

	router := gin.New()
	renderer := &stubHTMLRender{}
	router.HTMLRender = renderer
	router.Use(sessions.Sessions("commitlog_session", cookie.NewStore([]byte("test-secret"))))
	router.GET("/admin/dashboard", api.ShowDashboard)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !stubAnalytics.trendCalled {
		t.Fatal("expected HourlyTrafficTrend to be called")
	}
	if stubAnalytics.trendHours != 168 {
		t.Fatalf("expected HourlyTrafficTrend hours=168, got %d", stubAnalytics.trendHours)
	}
}

func TestShowDashboardIncludesLatestDraft(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gdb, cleanup := setupAdminHandlerTestDB(t)
	t.Cleanup(cleanup)

	user := db.User{Username: "dash-user"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	first := db.Post{
		Content: "# 第一篇\n内容",
		Status:  "draft",
		UserID:  user.ID,
	}
	second := db.Post{
		Content: "# 第二篇\n内容",
		Status:  "draft",
		UserID:  user.ID,
	}
	if err := gdb.Create(&first).Error; err != nil {
		t.Fatalf("failed to create first draft: %v", err)
	}
	if err := gdb.Create(&second).Error; err != nil {
		t.Fatalf("failed to create second draft: %v", err)
	}

	later := time.Now().Add(2 * time.Hour)
	if err := gdb.Model(&db.Post{}).
		Where("id = ?", first.ID).
		UpdateColumn("updated_at", later).Error; err != nil {
		t.Fatalf("failed to update first draft timestamp: %v", err)
	}

	api := &API{
		db:     gdb,
		posts:  service.NewPostService(gdb),
		system: service.NewSystemSettingService(gdb),
	}

	router := gin.New()
	renderer := &stubHTMLRender{}
	router.HTMLRender = renderer
	router.Use(sessions.Sessions("commitlog_session", cookie.NewStore([]byte("test-secret"))))
	router.GET("/admin/dashboard", api.ShowDashboard)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	payload, ok := renderer.lastData.(gin.H)
	if !ok {
		t.Fatalf("expected render payload to be gin.H, got %T", renderer.lastData)
	}

	latestRaw, exists := payload["latestDraft"]
	if !exists || latestRaw == nil {
		t.Fatalf("expected latestDraft in payload")
	}

	latest, ok := latestRaw.(*db.Post)
	if !ok {
		t.Fatalf("expected latestDraft to be *db.Post, got %T", latestRaw)
	}
	if latest.ID != first.ID {
		t.Fatalf("expected latest draft %d, got %d", first.ID, latest.ID)
	}
}

func TestShowDashboardIncludesCreationHeatmap(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gdb, cleanup := setupAdminHandlerTestDB(t)
	t.Cleanup(cleanup)

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}

	user := db.User{Username: "heatmap-admin"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	post := db.Post{
		Content:             "# 热力图文章\n内容",
		Status:              "published",
		UserID:              user.ID,
		CoverURL:            "https://example.com/cover.jpg",
		CoverWidth:          1200,
		CoverHeight:         800,
		PublicationCount:    2,
		LatestPublicationID: nil,
	}
	if err := gdb.Create(&post).Error; err != nil {
		t.Fatalf("failed to create post: %v", err)
	}

	firstPublishedAt := time.Date(2026, 2, 20, 10, 0, 0, 0, loc)
	firstPublication := db.PostPublication{
		PostID:      post.ID,
		Content:     post.Content,
		Visibility:  db.PostVisibilityPublic,
		UserID:      user.ID,
		PublishedAt: firstPublishedAt,
		Version:     1,
	}
	if err := gdb.Create(&firstPublication).Error; err != nil {
		t.Fatalf("failed to create first publication: %v", err)
	}

	secondPublishedAt := firstPublishedAt.Add(24 * time.Hour)
	secondPublication := db.PostPublication{
		PostID:      post.ID,
		Content:     "# 热力图文章（第二版）\n内容",
		Visibility:  db.PostVisibilityPublic,
		UserID:      user.ID,
		PublishedAt: secondPublishedAt,
		Version:     2,
	}
	if err := gdb.Create(&secondPublication).Error; err != nil {
		t.Fatalf("failed to create second publication: %v", err)
	}

	if err := gdb.Model(&db.Post{}).Where("id = ?", post.ID).Updates(map[string]interface{}{
		"latest_publication_id": secondPublication.ID,
		"published_at":          secondPublishedAt,
		"publication_count":     2,
	}).Error; err != nil {
		t.Fatalf("failed to update post publication pointers: %v", err)
	}

	api := &API{
		db:     gdb,
		posts:  service.NewPostService(gdb),
		system: service.NewSystemSettingService(gdb),
	}

	router := gin.New()
	renderer := &stubHTMLRender{}
	router.HTMLRender = renderer
	router.Use(sessions.Sessions("commitlog_session", cookie.NewStore([]byte("test-secret"))))
	router.GET("/admin/dashboard", api.ShowDashboard)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	payload, ok := renderer.lastData.(gin.H)
	if !ok {
		t.Fatalf("expected render payload to be gin.H, got %T", renderer.lastData)
	}

	raw, exists := payload["creationHeatmap"]
	if !exists {
		t.Fatal("expected creationHeatmap in payload")
	}

	heatmap, ok := raw.([]service.DailyCreationPoint)
	if !ok {
		t.Fatalf("expected creationHeatmap to be []service.DailyCreationPoint, got %T", raw)
	}
	if len(heatmap) != 365 {
		t.Fatalf("expected 365 heatmap points, got %d", len(heatmap))
	}

	creationDate := firstPublishedAt.Format("2006-01-02")
	creationCount := 0
	for _, point := range heatmap {
		if point.Date != creationDate {
			continue
		}
		creationCount = point.Count
		break
	}
	if creationCount != 1 {
		t.Fatalf("expected first publication day count 1, got %d", creationCount)
	}
}
