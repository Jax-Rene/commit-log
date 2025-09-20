package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func setupTestDB(t *testing.T) func() {
	t.Helper()

	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.User{}, &db.Post{}, &db.Tag{}, &db.Page{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
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

func TestCreatePostWithTags(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	tag := db.Tag{Name: "Go"}
	if err := db.DB.Create(&tag).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	payload := map[string]any{
		"title":        "Test Post",
		"content":      "Content",
		"summary":      "Summary",
		"status":       "draft",
		"tag_ids":      []uint{tag.ID},
		"cover_url":    "https://images.unsplash.com/photo-1500530855697-b586d89ba3ee",
		"cover_width":  1200,
		"cover_height": 800,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	CreatePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var created db.Post
	if err := db.DB.Preload("Tags").First(&created).Error; err != nil {
		t.Fatalf("failed to load created post: %v", err)
	}

	if created.Title != "Test Post" {
		t.Fatalf("unexpected title: %s", created.Title)
	}

	if len(created.Tags) != 1 || created.Tags[0].ID != tag.ID {
		t.Fatalf("expected associated tag with ID %d", tag.ID)
	}
}

func TestCreatePostRejectsUnknownTags(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	payload := map[string]any{
		"title":        "Test Post",
		"content":      "Content",
		"summary":      "Summary",
		"status":       "draft",
		"tag_ids":      []uint{99},
		"cover_url":    "https://images.unsplash.com/photo-1500530855697-b586d89ba3ee",
		"cover_width":  1200,
		"cover_height": 800,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	CreatePost(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestUpdatePostReplacesTags(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	originalTag := db.Tag{Name: "Go"}
	if err := db.DB.Create(&originalTag).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	replacementTag := db.Tag{Name: "Gin"}
	if err := db.DB.Create(&replacementTag).Error; err != nil {
		t.Fatalf("failed to seed replacement tag: %v", err)
	}

	post := db.Post{
		Title:       "Original",
		Content:     "Original content",
		Status:      "draft",
		UserID:      1,
		CoverURL:    "https://images.unsplash.com/photo-1498050108023-c5249f4df085",
		CoverWidth:  1280,
		CoverHeight: 720,
	}

	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	if err := db.DB.Model(&post).Association("Tags").Append(&originalTag); err != nil {
		t.Fatalf("failed to associate original tag: %v", err)
	}

	payload := map[string]any{
		"title":        "Updated",
		"content":      "Updated content",
		"summary":      "Updated summary",
		"status":       "published",
		"tag_ids":      []uint{replacementTag.ID},
		"cover_url":    "https://images.unsplash.com/photo-1523475472560-d2df97ec485c",
		"cover_width":  1440,
		"cover_height": 960,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/posts/"+strconv.Itoa(int(post.ID)), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(post.ID))}}

	UpdatePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var updated db.Post
	if err := db.DB.Preload("Tags").First(&updated, post.ID).Error; err != nil {
		t.Fatalf("failed to load updated post: %v", err)
	}

	if updated.Title != "Updated" {
		t.Fatalf("unexpected title: %s", updated.Title)
	}

	if len(updated.Tags) != 1 || updated.Tags[0].ID != replacementTag.ID {
		t.Fatalf("expected associated tag with ID %d", replacementTag.ID)
	}
}

func TestUpdatePostRejectsUnknownTags(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	post := db.Post{
		Title:       "Original",
		Content:     "Original content",
		Status:      "draft",
		UserID:      1,
		CoverURL:    "https://images.unsplash.com/photo-1498050108023-c5249f4df085",
		CoverWidth:  1280,
		CoverHeight: 720,
	}

	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	payload := map[string]any{
		"title":        "Updated",
		"content":      "Updated content",
		"summary":      "Updated summary",
		"status":       "published",
		"tag_ids":      []uint{123},
		"cover_url":    "https://images.unsplash.com/photo-1523475472560-d2df97ec485c",
		"cover_width":  1440,
		"cover_height": 960,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/posts/"+strconv.Itoa(int(post.ID)), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(post.ID))}}

	UpdatePost(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestUpdatePostClearsTagsWhenEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	tag := db.Tag{Name: "Go"}
	if err := db.DB.Create(&tag).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	post := db.Post{
		Title:       "Original",
		Content:     "Original content",
		Status:      "draft",
		UserID:      1,
		CoverURL:    "https://images.unsplash.com/photo-1498050108023-c5249f4df085",
		CoverWidth:  1280,
		CoverHeight: 720,
	}

	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	if err := db.DB.Model(&post).Association("Tags").Append(&tag); err != nil {
		t.Fatalf("failed to associate tag: %v", err)
	}

	payload := map[string]any{
		"title":        "Updated",
		"content":      "Updated content",
		"summary":      "Updated summary",
		"status":       "published",
		"tag_ids":      []uint{},
		"cover_url":    "https://images.unsplash.com/photo-1500530855697-b586d89ba3ee",
		"cover_width":  1200,
		"cover_height": 800,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/posts/"+strconv.Itoa(int(post.ID)), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(post.ID))}}

	UpdatePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var updated db.Post
	if err := db.DB.Preload("Tags").First(&updated, post.ID).Error; err != nil {
		t.Fatalf("failed to load updated post: %v", err)
	}

	if len(updated.Tags) != 0 {
		t.Fatalf("expected no tags, found %d", len(updated.Tags))
	}
}

func TestDeletePost(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	post := db.Post{
		Title:       "Delete Me",
		Content:     "Content",
		Status:      "draft",
		UserID:      1,
		CoverURL:    "https://images.unsplash.com/photo-1498050108023-c5249f4df085",
		CoverWidth:  1280,
		CoverHeight: 720,
	}

	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/posts/"+strconv.Itoa(int(post.ID)), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(post.ID))}}

	DeletePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var count int64
	db.DB.Model(&db.Post{}).Where("id = ?", post.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected post to be deleted, still found %d records", count)
	}
}
