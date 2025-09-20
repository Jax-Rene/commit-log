package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/router"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ginOnce sync.Once

func setupPublicTestDB(t *testing.T) func() {
	t.Helper()

	ginOnce.Do(func() {
		gin.SetMode(gin.TestMode)
	})

	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.User{}, &db.Post{}, &db.Tag{}, &db.Page{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	if err := gdb.Create(&db.User{Username: "tester", Password: "hashed"}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	db.DB = gdb

	return func() {
		sqlDB, err := db.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestShowHomeExcludesDrafts(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	published := db.Post{Title: "Published Post", Content: "内容", Summary: "摘要", Status: "published", UserID: 1}
	published.CoverURL = "https://images.unsplash.com/photo-1472214103451-9374bd1c798e"
	published.CoverWidth = 1280
	published.CoverHeight = 720
	if err := db.DB.Create(&published).Error; err != nil {
		t.Fatalf("failed to create published post: %v", err)
	}

	draft := db.Post{Title: "Draft Post", Content: "草稿", Status: "draft", UserID: 1}
	draft.CoverURL = "https://images.unsplash.com/photo-1441986300917-64674bd600d8"
	draft.CoverWidth = 800
	draft.CoverHeight = 1200
	if err := db.DB.Create(&draft).Error; err != nil {
		t.Fatalf("failed to create draft post: %v", err)
	}

	r := router.SetupRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, published.Title) {
		t.Fatalf("expected response to include published post title")
	}
	if strings.Contains(body, draft.Title) {
		t.Fatalf("draft post should not be rendered on public home")
	}
}

func TestLoadMorePostsHandlesPagination(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	for i := 1; i <= 7; i++ {
		post := db.Post{Title: "Post " + strconv.Itoa(i), Content: "内容", Status: "published", UserID: 1}
		post.CoverURL = "https://images.unsplash.com/photo-1523475472560-d2df97ec485c?sig=" + strconv.Itoa(i)
		post.CoverWidth = 1200 + i*10
		post.CoverHeight = 800 + i*5
		if err := db.DB.Create(&post).Error; err != nil {
			t.Fatalf("failed to seed post %d: %v", i, err)
		}
	}

	r := router.SetupRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/more?page=2", nil)
	req.Header.Set("HX-Request", "true")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Post 1") {
		t.Fatalf("expected response to include paginated posts")
	}
}

func TestShowPostDetailRejectsDraft(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	draft := db.Post{Title: "Drafted", Content: "草稿", Status: "draft", UserID: 1}
	draft.CoverURL = "https://images.unsplash.com/photo-1487412720507-e7ab37603c6f"
	draft.CoverWidth = 1080
	draft.CoverHeight = 1350
	if err := db.DB.Create(&draft).Error; err != nil {
		t.Fatalf("failed to create draft: %v", err)
	}

	r := router.SetupRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+strconv.Itoa(int(draft.ID)), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for draft post, got %d", w.Code)
	}
}

func TestShowAboutFallback(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	r := router.SetupRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "关于我") {
		t.Fatalf("expected fallback about title in response")
	}
}
