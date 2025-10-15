package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupPostServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:post-service-%d?mode=memory&cache=shared", time.Now().UnixNano())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.User{}, &db.Tag{}, &db.Post{}, &db.PostPublication{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	return gdb
}

func TestPostService_PublishFlow(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "tester"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	tag := db.Tag{Name: "Go"}
	if err := gdb.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}

	input := PostInput{
		Title:       "测试文章",
		Content:     "这是正文内容，用于计算阅读时长。",
		Summary:     "文章摘要",
		TagIDs:      []uint{tag.ID},
		UserID:      user.ID,
		CoverURL:    "https://example.com/cover.jpg",
		CoverWidth:  1200,
		CoverHeight: 800,
	}

	post, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if post.Status != "draft" {
		t.Fatalf("expected draft status after create, got %s", post.Status)
	}

	publication, err := svc.Publish(post.ID, user.ID)
	if err != nil {
		t.Fatalf("publish post: %v", err)
	}

	if publication.PostID != post.ID {
		t.Fatalf("publication post id mismatch: %d vs %d", publication.PostID, post.ID)
	}
	if publication.Version != 1 {
		t.Fatalf("expected version 1, got %d", publication.Version)
	}
	if publication.PublishedAt.IsZero() {
		t.Fatalf("expected published at to be set")
	}
	if publication.ReadingTime <= 0 {
		t.Fatalf("expected positive reading time")
	}
	if len(publication.Tags) != 1 {
		t.Fatalf("expected 1 tag on publication, got %d", len(publication.Tags))
	}

	stored, err := svc.Get(post.ID)
	if err != nil {
		t.Fatalf("fetch post: %v", err)
	}
	if stored.Status != "published" {
		t.Fatalf("expected status published, got %s", stored.Status)
	}
	if stored.PublicationCount != 1 {
		t.Fatalf("expected publication count 1, got %d", stored.PublicationCount)
	}
	if stored.PublishedAt.IsZero() {
		t.Fatalf("post published at not set")
	}
	if stored.LatestPublicationID == nil {
		t.Fatalf("expected latest publication id to be set")
	}

	latest, err := svc.LatestPublication(post.ID)
	if err != nil {
		t.Fatalf("latest publication: %v", err)
	}
	if latest.ID != publication.ID {
		t.Fatalf("expected latest publication id %d, got %d", publication.ID, latest.ID)
	}

	list, err := svc.ListPublished(PostFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list published: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("expected total 1, got %d", list.Total)
	}
	if len(list.Publications) != 1 || list.Publications[0].ID != publication.ID {
		t.Fatalf("expected latest publication in list")
	}

	// Create second version
	updatedInput := input
	updatedInput.Content = "更新后的正文内容，包含更多文字用于新的版本。"
	if _, err := svc.Update(post.ID, updatedInput); err != nil {
		t.Fatalf("update post before republish: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	publication2, err := svc.Publish(post.ID, user.ID)
	if err != nil {
		t.Fatalf("publish second version: %v", err)
	}
	if publication2.Version != 2 {
		t.Fatalf("expected version 2, got %d", publication2.Version)
	}

	latest, err = svc.LatestPublication(post.ID)
	if err != nil {
		t.Fatalf("latest publication after republish: %v", err)
	}
	if latest.ID != publication2.ID {
		t.Fatalf("expected latest publication id %d, got %d", publication2.ID, latest.ID)
	}

	list, err = svc.ListPublished(PostFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list published after republish: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("expected total 1 after republish, got %d", list.Total)
	}
	if len(list.Publications) != 1 || list.Publications[0].ID != publication2.ID {
		t.Fatalf("expected only latest publication in list")
	}

	// Tag filter
	filtered, err := svc.ListPublished(PostFilter{Page: 1, PerPage: 10, TagNames: []string{"Go"}})
	if err != nil {
		t.Fatalf("list published with tag: %v", err)
	}
	if filtered.Total != 1 {
		t.Fatalf("expected total 1 with tag filter, got %d", filtered.Total)
	}

	filteredNone, err := svc.ListPublished(PostFilter{Page: 1, PerPage: 10, TagNames: []string{"Unknown"}})
	if err != nil {
		t.Fatalf("list published with unknown tag: %v", err)
	}
	if filteredNone.Total != 0 || len(filteredNone.Publications) != 0 {
		t.Fatalf("expected no publications for unknown tag")
	}
}
