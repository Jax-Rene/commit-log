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

	if err := gdb.AutoMigrate(&db.User{}, &db.Tag{}, &db.Post{}, &db.PostPublication{}, &db.PostDraftVersion{}, &db.PostStatistic{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	return gdb
}

func TestPostService_ListCountsDrafts(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "counter-tester"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := svc.Create(PostInput{
		Content: "# 草稿标题\n草稿正文",
		Summary: "草稿摘要",
		UserID:  user.ID,
	}); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	published, err := svc.Create(PostInput{
		Content:     "# 已发布标题\n发布内容",
		Summary:     "发布摘要",
		UserID:      user.ID,
		CoverURL:    "https://example.com/cover.jpg",
		CoverWidth:  1200,
		CoverHeight: 800,
	})
	if err != nil {
		t.Fatalf("create publishable post: %v", err)
	}
	if _, err := svc.Publish(published.ID, user.ID, nil); err != nil {
		t.Fatalf("publish post: %v", err)
	}

	list, err := svc.List(PostFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list posts: %v", err)
	}
	if list.Total != 2 {
		t.Fatalf("expected total 2, got %d", list.Total)
	}
	if list.PublishedCount != 1 {
		t.Fatalf("expected published count 1, got %d", list.PublishedCount)
	}
	if list.DraftCount != 1 {
		t.Fatalf("expected draft count 1, got %d", list.DraftCount)
	}
}

func TestPostService_ListDefaultsToCreatedDesc(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "sort-default"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	input := PostInput{
		Content:     "# 第一篇\n内容",
		Summary:     "摘要",
		UserID:      user.ID,
		CoverURL:    "https://example.com/cover.jpg",
		CoverWidth:  1200,
		CoverHeight: 800,
	}

	first, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create first post: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	secondInput := input
	secondInput.Content = "# 第二篇\n内容"
	second, err := svc.Create(secondInput)
	if err != nil {
		t.Fatalf("create second post: %v", err)
	}

	publishedAtFirst := time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)
	publishedAtSecond := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	if _, err := svc.Publish(first.ID, user.ID, &publishedAtFirst); err != nil {
		t.Fatalf("publish first post: %v", err)
	}
	if _, err := svc.Publish(second.ID, user.ID, &publishedAtSecond); err != nil {
		t.Fatalf("publish second post: %v", err)
	}

	list, err := svc.List(PostFilter{Page: 1, PerPage: 10, Status: "published"})
	if err != nil {
		t.Fatalf("list posts: %v", err)
	}
	if len(list.Posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(list.Posts))
	}
	if list.Posts[0].ID != second.ID {
		t.Fatalf("expected newest created post first, got %d", list.Posts[0].ID)
	}
}

func TestPostService_LatestDraftReturnsMostRecentEdit(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "latest-draft"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	first, err := svc.Create(PostInput{
		Content: "# 第一稿\n内容",
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("create first draft: %v", err)
	}

	second, err := svc.Create(PostInput{
		Content: "# 第二稿\n内容",
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("create second draft: %v", err)
	}

	later := time.Now().Add(2 * time.Hour)
	if err := gdb.Model(&db.Post{}).
		Where("id = ?", first.ID).
		UpdateColumn("updated_at", later).Error; err != nil {
		t.Fatalf("update first draft updated_at: %v", err)
	}

	latest, err := svc.LatestDraft(user.ID)
	if err != nil {
		t.Fatalf("latest draft: %v", err)
	}
	if latest.ID != first.ID {
		t.Fatalf("expected latest draft %d, got %d", first.ID, latest.ID)
	}
	if latest.ID == second.ID {
		t.Fatalf("expected latest draft to ignore created time ordering")
	}
}

func TestPostService_LatestDraftFiltersByUser(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "primary"}
	other := db.User{Username: "secondary"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := gdb.Create(&other).Error; err != nil {
		t.Fatalf("create other user: %v", err)
	}

	userDraft, err := svc.Create(PostInput{
		Content: "# 用户草稿\n内容",
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("create user draft: %v", err)
	}

	otherDraft, err := svc.Create(PostInput{
		Content: "# 其他草稿\n内容",
		UserID:  other.ID,
	})
	if err != nil {
		t.Fatalf("create other draft: %v", err)
	}

	later := time.Now().Add(3 * time.Hour)
	if err := gdb.Model(&db.Post{}).
		Where("id = ?", otherDraft.ID).
		UpdateColumn("updated_at", later).Error; err != nil {
		t.Fatalf("update other draft updated_at: %v", err)
	}

	latest, err := svc.LatestDraft(user.ID)
	if err != nil {
		t.Fatalf("latest draft: %v", err)
	}
	if latest.ID != userDraft.ID {
		t.Fatalf("expected user latest draft %d, got %d", userDraft.ID, latest.ID)
	}
}

func TestPostService_LatestDraftReturnsNotFoundWhenEmpty(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "empty-draft"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := svc.LatestDraft(user.ID); !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("expected ErrPostNotFound, got %v", err)
	}
}

func TestPostService_ListPublishedSplitsSearchTokens(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "search-split"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	post, err := svc.Create(PostInput{
		Content:     "# Hello\nWorld in body",
		Summary:     "摘要",
		UserID:      user.ID,
		CoverURL:    "https://example.com/cover.jpg",
		CoverWidth:  1200,
		CoverHeight: 800,
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if _, err := svc.Publish(post.ID, user.ID, nil); err != nil {
		t.Fatalf("publish post: %v", err)
	}

	list, err := svc.ListPublished(PostFilter{
		Search:  "Hello World",
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("list published posts: %v", err)
	}
	if len(list.Publications) != 1 {
		t.Fatalf("expected 1 publication, got %d", len(list.Publications))
	}
}

func TestPostService_ListSortsByHot(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "sort-hot"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	hotPost, err := svc.Create(PostInput{
		Content: "# 热门\n内容",
		Summary: "摘要",
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("create hot post: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	coldPost, err := svc.Create(PostInput{
		Content: "# 冷门\n内容",
		Summary: "摘要",
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("create cold post: %v", err)
	}

	if err := gdb.Create(&db.PostStatistic{
		PostID:         hotPost.ID,
		PageViews:      120,
		UniqueVisitors: 40,
	}).Error; err != nil {
		t.Fatalf("create hot stats: %v", err)
	}
	if err := gdb.Create(&db.PostStatistic{
		PostID:         coldPost.ID,
		PageViews:      5,
		UniqueVisitors: 2,
	}).Error; err != nil {
		t.Fatalf("create cold stats: %v", err)
	}

	list, err := svc.List(PostFilter{Page: 1, PerPage: 10, Sort: "hot"})
	if err != nil {
		t.Fatalf("list hot posts: %v", err)
	}
	if len(list.Posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(list.Posts))
	}
	if list.Posts[0].ID != hotPost.ID {
		t.Fatalf("expected hottest post first, got %d", list.Posts[0].ID)
	}
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

	publication, err := svc.Publish(post.ID, user.ID, nil)
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

	publication2, err := svc.Publish(post.ID, user.ID, nil)
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

func TestPostService_DraftVersionsKeepLatestTen(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "draft-versioner"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	post, err := svc.Create(PostInput{
		Content: "# v1\n内容",
		Summary: "s1",
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	for i := 2; i <= 12; i++ {
		content := fmt.Sprintf("# v%d\n内容", i)
		summary := fmt.Sprintf("s%d", i)
		if _, err := svc.Update(post.ID, PostInput{
			Content: content,
			Summary: summary,
			UserID:  user.ID,
		}); err != nil {
			t.Fatalf("update post version %d: %v", i, err)
		}
	}

	versions, err := svc.ListDraftVersions(post.ID, 0)
	if err != nil {
		t.Fatalf("list draft versions: %v", err)
	}
	if len(versions) != 10 {
		t.Fatalf("expected 10 draft versions, got %d", len(versions))
	}
	if versions[0].Version != 12 {
		t.Fatalf("expected latest version 12, got %d", versions[0].Version)
	}
	last := versions[len(versions)-1]
	if last.Version != 3 {
		t.Fatalf("expected oldest kept version 3, got %d", last.Version)
	}
}

func TestPostService_DraftVersionSkipsWhenContentUnchanged(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "draft-skipper"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	post, err := svc.Create(PostInput{
		Content: "# v1\n内容",
		Summary: "s1",
		UserID:  user.ID,
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	if _, err := svc.Update(post.ID, PostInput{
		Content: "# v1\n内容",
		Summary: "s1",
		UserID:  user.ID,
	}); err != nil {
		t.Fatalf("update post with same content: %v", err)
	}

	versions, err := svc.ListDraftVersions(post.ID, 0)
	if err != nil {
		t.Fatalf("list draft versions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 draft version, got %d", len(versions))
	}

	if _, err := svc.Update(post.ID, PostInput{
		Content: "# v2\n内容",
		Summary: "s2",
		UserID:  user.ID,
	}); err != nil {
		t.Fatalf("update post with new content: %v", err)
	}

	versions, err = svc.ListDraftVersions(post.ID, 0)
	if err != nil {
		t.Fatalf("list draft versions after change: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 draft versions, got %d", len(versions))
	}
}

func TestPostService_DraftVersionGroupsBySession(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "session-writer"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	post, err := svc.Create(PostInput{
		Content:        "# v1\n内容",
		Summary:        "s1",
		UserID:         user.ID,
		DraftSessionID: "session-a",
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	if _, err := svc.Update(post.ID, PostInput{
		Content:        "# v2\n内容",
		Summary:        "s2",
		UserID:         user.ID,
		DraftSessionID: "session-a",
	}); err != nil {
		t.Fatalf("update post in same session: %v", err)
	}

	versions, err := svc.ListDraftVersions(post.ID, 0)
	if err != nil {
		t.Fatalf("list draft versions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 draft version for same session, got %d", len(versions))
	}
	if versions[0].Content != "# v2\n内容" {
		t.Fatalf("expected latest content in session, got %q", versions[0].Content)
	}
	if versions[0].SessionID != "session-a" {
		t.Fatalf("expected session id session-a, got %q", versions[0].SessionID)
	}

	if _, err := svc.Update(post.ID, PostInput{
		Content:        "# v3\n内容",
		Summary:        "s3",
		UserID:         user.ID,
		DraftSessionID: "session-b",
	}); err != nil {
		t.Fatalf("update post in new session: %v", err)
	}

	versions, err = svc.ListDraftVersions(post.ID, 0)
	if err != nil {
		t.Fatalf("list draft versions after new session: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 draft versions after new session, got %d", len(versions))
	}
	if versions[0].SessionID != "session-b" {
		t.Fatalf("expected latest session id session-b, got %q", versions[0].SessionID)
	}
	if versions[0].Version != 2 {
		t.Fatalf("expected new version number 2, got %d", versions[0].Version)
	}
	if versions[1].SessionID != "session-a" {
		t.Fatalf("expected previous session id session-a, got %q", versions[1].SessionID)
	}
}

func TestPostService_ListPublishedOrderedByPublishTime(t *testing.T) {
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
	if _, err := svc.Publish(first.ID, user.ID, nil); err != nil {
		t.Fatalf("publish first post: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	secondInput := input
	secondInput.Content = "第二篇文章"
	second, err := svc.Create(secondInput)
	if err != nil {
		t.Fatalf("create second post: %v", err)
	}
	if _, err := svc.Publish(second.ID, user.ID, nil); err != nil {
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

	if _, err := svc.Publish(first.ID, user.ID, nil); err != nil {
		t.Fatalf("republish first post: %v", err)
	}

	list, err = svc.ListPublished(PostFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list published after republish: %v", err)
	}
	if list.Publications[0].PostID != first.ID {
		t.Fatalf("expected republished post to move to top by published_at, got %d", list.Publications[0].PostID)
	}
}

func TestPostService_PublishWithCustomPublishedAt(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	svc := NewPostService(gdb)

	user := db.User{Username: "custom-publish"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	post, err := svc.Create(PostInput{Content: "内容", Summary: "摘要", UserID: user.ID, CoverURL: "https://example.com/c.jpg", CoverWidth: 600, CoverHeight: 400})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	customPublishedAt := time.Date(2023, 8, 15, 12, 30, 0, 0, time.UTC)
	publication, err := svc.Publish(post.ID, user.ID, &customPublishedAt)
	if err != nil {
		t.Fatalf("publish with custom time: %v", err)
	}

	if !publication.PublishedAt.Equal(customPublishedAt) {
		t.Fatalf("expected publication published_at %v, got %v", customPublishedAt, publication.PublishedAt)
	}

	refreshed, err := svc.Get(post.ID)
	if err != nil {
		t.Fatalf("reload post: %v", err)
	}
	if !refreshed.PublishedAt.Equal(customPublishedAt) {
		t.Fatalf("expected post published_at %v, got %v", customPublishedAt, refreshed.PublishedAt)
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

	if _, err := svc.Publish(post.ID, user.ID, nil); !errors.Is(err, ErrCoverRequired) {
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

func TestCalculateReadingTimeIgnoresLinks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "hyperlink text preserved url removed",
			content:  "这是一个包含链接的内容，[点击这里](https://example.com/very/long/path?with=query)。",
			expected: 1,
		},
		{
			name:     "image link removed",
			content:  "前言 ![示例图片](https://example.com/image.png) 结尾",
			expected: 1,
		},
		{
			name:     "bare url removed",
			content:  "请访问 https://example.com/path 了解详情",
			expected: 1,
		},
		{
			name:     "only url returns zero",
			content:  "https://example.com/path",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateReadingTime(tt.content)
			if got != tt.expected {
				t.Fatalf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}
