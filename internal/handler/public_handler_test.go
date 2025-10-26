package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

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

	dsn := fmt.Sprintf("file:public-handler-%d?mode=memory&cache=shared", time.Now().UnixNano())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.User{}, &db.Post{}, &db.PostPublication{}, &db.Tag{}, &db.Page{}, &db.ProfileContact{}, &db.PostStatistic{}, &db.PostVisit{}, &db.SystemSetting{}); err != nil {
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

func seedPublishedPostAt(t *testing.T, title, content string, publishedAt time.Time) db.Post {
	t.Helper()

	summary := fmt.Sprintf("%s 摘要", title)
	post := db.Post{
		Title:       title,
		Content:     content,
		Summary:     summary,
		Status:      "draft",
		UserID:      1,
		CoverURL:    fmt.Sprintf("https://images.unsplash.com/photo-1500530855697-b586d89ba3ee?title=%s", urlSafe(title)),
		CoverWidth:  1280,
		CoverHeight: 720,
	}
	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to create post: %v", err)
	}

	publication := db.PostPublication{
		PostID:      post.ID,
		Title:       post.Title,
		Content:     post.Content,
		Summary:     post.Summary,
		ReadingTime: 1,
		CoverURL:    post.CoverURL,
		CoverWidth:  post.CoverWidth,
		CoverHeight: post.CoverHeight,
		UserID:      post.UserID,
		PublishedAt: publishedAt,
		Version:     1,
	}
	if err := db.DB.Create(&publication).Error; err != nil {
		t.Fatalf("failed to create publication: %v", err)
	}

	updates := map[string]any{
		"status":                "published",
		"published_at":          publication.PublishedAt,
		"publication_count":     1,
		"latest_publication_id": publication.ID,
	}
	if err := db.DB.Model(&post).Updates(updates).Error; err != nil {
		t.Fatalf("failed to update post metadata: %v", err)
	}

	if err := db.DB.First(&post, post.ID).Error; err != nil {
		t.Fatalf("failed to reload post: %v", err)
	}

	return post
}

func seedPublishedPost(t *testing.T, title, content string) db.Post {
	return seedPublishedPostAt(t, title, content, time.Now())
}

func seedDraftPost(t *testing.T, title string) db.Post {
	t.Helper()
	post := db.Post{
		Title:       title,
		Content:     "草稿内容",
		Status:      "draft",
		UserID:      1,
		CoverURL:    fmt.Sprintf("https://images.unsplash.com/photo-1441986300917-64674bd600d8?title=%s", urlSafe(title)),
		CoverWidth:  960,
		CoverHeight: 1280,
	}
	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to create draft: %v", err)
	}
	return post
}

func urlSafe(input string) string {
	return strings.ReplaceAll(input, " ", "+")
}

func TestShowHomeExcludesDrafts(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	published := seedPublishedPost(t, "Published Post", "内容")
	draft := seedDraftPost(t, "Draft Post")

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads")
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

	now := time.Now()
	for i := 1; i <= 7; i++ {
		title := "Post " + strconv.Itoa(i)
		seedPublishedPostAt(t, title, "内容", now.Add(-time.Duration(i)*time.Minute))
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/more?page=2", nil)
	req.Header.Set("HX-Request", "true")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Post 7") {
		t.Fatalf("expected paginated response to include remaining posts")
	}
	if strings.Contains(body, "Post 1") {
		t.Fatalf("expected second page to exclude first page items")
	}
}

func TestShowPostDetailRejectsDraft(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	draft := seedDraftPost(t, "Drafted")

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads")
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

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads")
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

func TestShowAboutDisplaysContacts(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	aboutPage := db.Page{Slug: "about", Title: "关于我", Content: "# 你好"}
	if err := db.DB.Create(&aboutPage).Error; err != nil {
		t.Fatalf("failed to seed about page: %v", err)
	}

	contacts := []db.ProfileContact{
		{Platform: "微信", Label: "个人微信", Value: "coder-123", Icon: "wechat", Sort: 0, Visible: true},
		{Platform: "GitHub", Label: "GitHub", Value: "https://github.com/commitlog", Link: "https://github.com/commitlog", Icon: "github", Sort: 1, Visible: true},
	}
	if err := db.DB.Create(&contacts).Error; err != nil {
		t.Fatalf("failed to seed contacts: %v", err)
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "个人微信") {
		t.Fatalf("expected contact label to render")
	}
	if !strings.Contains(body, "https://github.com/commitlog") {
		t.Fatalf("expected contact link to render")
	}
	if !strings.Contains(body, "联系我") {
		t.Fatalf("expected contact section heading")
	}
}

func TestShowPostDetailDisplaysContacts(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	post := seedPublishedPost(t, "公开文章", "## 内容")

	contact := db.ProfileContact{Platform: "邮箱", Label: "联系邮箱", Value: "hi@example.com", Link: "mailto:hi@example.com", Icon: "email", Sort: 0, Visible: true}
	if err := db.DB.Create(&contact).Error; err != nil {
		t.Fatalf("failed to seed contact: %v", err)
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+strconv.Itoa(int(post.ID)), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "联系作者") {
		t.Fatalf("expected contact banner heading")
	}
	if !strings.Contains(body, "mailto:hi@example.com") {
		t.Fatalf("expected contact link to render")
	}
}

func TestShowPostDetailStripsLeadingTitleFromContent(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	content := "# 公开文章\n\n正文段落"
	post := seedPublishedPost(t, "公开文章", content)

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+strconv.Itoa(int(post.ID)), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, "# 公开文章") {
		t.Fatalf("expected rendered content to exclude leading markdown title")
	}
	if !strings.Contains(body, "正文段落") {
		t.Fatalf("expected rendered content to retain body text")
	}
}
