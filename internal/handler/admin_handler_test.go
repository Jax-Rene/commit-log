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

type stubHTMLRender struct{}

type stubHTMLInstance struct {
	name string
	data interface{}
}

func (r *stubHTMLRender) Instance(name string, data interface{}) render.Render {
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

	if err := gdb.AutoMigrate(&db.Post{}, &db.Tag{}, &db.SystemSetting{}); err != nil {
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
		system:    service.NewSystemSettingService(gdb),
		analytics: stubAnalytics,
	}

	router := gin.New()
	router.HTMLRender = &stubHTMLRender{}
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
