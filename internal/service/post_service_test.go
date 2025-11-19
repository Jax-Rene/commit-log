package service

import (
	"errors"
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

func TestPostService_ListPublishedOrderedByCreationTime(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "order-tester"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	tag := db.Tag{Name: "General"}
	if err := gdb.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}

	input := PostInput{
		Title:       "Post",
		Content:     "第一篇文章",
		Summary:     "摘要",
		TagIDs:      []uint{tag.ID},
		UserID:      user.ID,
		CoverURL:    "https://example.com/cover.jpg",
		CoverWidth:  800,
		CoverHeight: 600,
	}

	first, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create first post: %v", err)
	}
	if _, err := svc.Publish(first.ID, user.ID); err != nil {
		t.Fatalf("publish first post: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	secondInput := input
	secondInput.Content = "第二篇文章"
	second, err := svc.Create(secondInput)
	if err != nil {
		t.Fatalf("create second post: %v", err)
	}
	if _, err := svc.Publish(second.ID, user.ID); err != nil {
		t.Fatalf("publish second post: %v", err)
	}

	list, err := svc.ListPublished(PostFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list published: %v", err)
	}
	if len(list.Publications) != 2 {
		t.Fatalf("expected 2 publications, got %d", len(list.Publications))
	}
	if list.Publications[0].PostID != second.ID {
		t.Fatalf("expected second created post first, got %d", list.Publications[0].PostID)
	}

	time.Sleep(10 * time.Millisecond)

	if _, err := svc.Publish(first.ID, user.ID); err != nil {
		t.Fatalf("republish first post: %v", err)
	}

	list, err = svc.ListPublished(PostFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list published after republish: %v", err)
	}
	if list.Publications[0].PostID != second.ID {
		t.Fatalf("expected ordering by creation time to keep second first, got %d", list.Publications[0].PostID)
	}
}

func TestPostService_CreateAndUpdateWithoutCover(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "draft-writer"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	input := PostInput{
		Content: "# 草稿允许无封面\n内容",
		Summary: "摘要",
		UserID:  user.ID,
	}

	post, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create post without cover: %v", err)
	}
	if post.CoverURL != "" {
		t.Fatalf("expected empty cover url, got %s", post.CoverURL)
	}
	if post.CoverWidth != 0 || post.CoverHeight != 0 {
		t.Fatalf("expected zero cover dimensions, got %dx%d", post.CoverWidth, post.CoverHeight)
	}

	update := input
	update.Content = "# 更新后的标题\n更多内容"
	updated, err := svc.Update(post.ID, update)
	if err != nil {
		t.Fatalf("update post without cover: %v", err)
	}
	if updated.Title != "更新后的标题" {
		t.Fatalf("unexpected title after update: %s", updated.Title)
	}
	if updated.CoverURL != "" {
		t.Fatalf("expected empty cover url after update, got %s", updated.CoverURL)
	}
	if updated.CoverWidth != 0 || updated.CoverHeight != 0 {
		t.Fatalf("expected zero cover dimensions after update, got %dx%d", updated.CoverWidth, updated.CoverHeight)
	}
}

func TestPostService_PublishRequiresCover(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "publisher"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	input := PostInput{
		Content: "# 缺少封面\n正文",
		Summary: "摘要",
		UserID:  user.ID,
	}

	post, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create draft without cover: %v", err)
	}

	if _, err := svc.Publish(post.ID, user.ID); !errors.Is(err, ErrCoverRequired) {
		t.Fatalf("expected ErrCoverRequired when publishing without cover, got %v", err)
	}
}

func TestPostService_DeriveTitleFromContent(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "auto-title"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	post, err := svc.Create(PostInput{
		Content: "   # 自动标题  \n这是一段正文", // first heading should become title
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("create post with derived title: %v", err)
	}
	if post.Title != "自动标题" {
		t.Fatalf("expected derived title '自动标题', got %q", post.Title)
	}

	updated, err := svc.Update(post.ID, PostInput{
		Content: "# 新的标题\n更新的正文",
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("update post with derived title: %v", err)
	}
	if updated.Title != "新的标题" {
		t.Fatalf("expected derived title '新的标题', got %q", updated.Title)
	}

	// When没有可用的首行标题时，保持现有标题
	preserved, err := svc.Update(post.ID, PostInput{
		Content: "正文没有标题\n# 二级标题",
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("update post without heading: %v", err)
	}
	if preserved.Title != "正文没有标题" {
		t.Fatalf("expected title to fall back to first line, got %q", preserved.Title)
	}
}
