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

func setupTagServiceTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	dsn := fmt.Sprintf("file:tag-service-%d?mode=memory&cache=shared", time.Now().UnixNano())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := gdb.AutoMigrate(&db.Tag{}, &db.Post{}, &db.PostPublication{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return gdb, func() {
		sqlDB, err := gdb.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
}

func TestTagServiceCreateAssignsNextSortOrder(t *testing.T) {
	gdb, cleanup := setupTagServiceTestDB(t)
	defer cleanup()

	if err := gdb.Create(&db.Tag{Name: "已有标签", SortOrder: 5}).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	svc := NewTagService(gdb)
	tag, err := svc.Create("新标签")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	if tag.SortOrder != 6 {
		t.Fatalf("expected sort_order=6, got %d", tag.SortOrder)
	}
}

func TestTagServiceListOrdersBySortOrder(t *testing.T) {
	gdb, cleanup := setupTagServiceTestDB(t)
	defer cleanup()

	tags := []db.Tag{
		{Name: "Zed", SortOrder: 0},
		{Name: "Alpha", SortOrder: 2},
		{Name: "Beta", SortOrder: 1},
	}
	if err := gdb.Create(&tags).Error; err != nil {
		t.Fatalf("failed to seed tags: %v", err)
	}

	svc := NewTagService(gdb)
	list, err := svc.List()
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(list))
	}

	if list[0].Name != "Zed" || list[1].Name != "Beta" || list[2].Name != "Alpha" {
		t.Fatalf("unexpected order: %+v", []string{list[0].Name, list[1].Name, list[2].Name})
	}
}

func TestTagServiceReorderUpdatesSortOrder(t *testing.T) {
	gdb, cleanup := setupTagServiceTestDB(t)
	defer cleanup()

	tags := []db.Tag{
		{Name: "A", SortOrder: 0},
		{Name: "B", SortOrder: 1},
		{Name: "C", SortOrder: 2},
	}
	if err := gdb.Create(&tags).Error; err != nil {
		t.Fatalf("failed to seed tags: %v", err)
	}

	svc := NewTagService(gdb)
	if err := svc.Reorder([]uint{tags[2].ID, tags[0].ID, tags[1].ID}); err != nil {
		t.Fatalf("reorder tags: %v", err)
	}

	list, err := svc.List()
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}

	if list[0].Name != "C" || list[1].Name != "A" || list[2].Name != "B" {
		t.Fatalf("unexpected order after reorder: %+v", []string{list[0].Name, list[1].Name, list[2].Name})
	}
	if list[0].SortOrder != 0 || list[1].SortOrder != 1 || list[2].SortOrder != 2 {
		t.Fatalf("unexpected sort_order after reorder: %+v", []int{list[0].SortOrder, list[1].SortOrder, list[2].SortOrder})
	}
}

func TestTagService_PublishedUsageExcludesUnlistedPosts(t *testing.T) {
	gdb, cleanup := setupTagServiceTestDB(t)
	defer cleanup()

	publicTag := db.Tag{Name: "公开标签", SortOrder: 0}
	unlistedTag := db.Tag{Name: "隐藏标签", SortOrder: 1}
	if err := gdb.Create(&publicTag).Error; err != nil {
		t.Fatalf("create public tag: %v", err)
	}
	if err := gdb.Create(&unlistedTag).Error; err != nil {
		t.Fatalf("create unlisted tag: %v", err)
	}

	publicPost := db.Post{
		Content:    "# 公开\n正文",
		Status:     "published",
		UserID:     1,
		Visibility: db.PostVisibilityPublic,
	}
	if err := gdb.Create(&publicPost).Error; err != nil {
		t.Fatalf("create public post: %v", err)
	}
	publicPublication := db.PostPublication{
		PostID:      publicPost.ID,
		Content:     publicPost.Content,
		UserID:      1,
		PublishedAt: time.Now().Add(-time.Hour),
		Version:     1,
		Visibility:  db.PostVisibilityPublic,
	}
	if err := gdb.Create(&publicPublication).Error; err != nil {
		t.Fatalf("create public publication: %v", err)
	}
	if err := gdb.Model(&publicPost).Update("latest_publication_id", publicPublication.ID).Error; err != nil {
		t.Fatalf("update public latest publication: %v", err)
	}
	if err := gdb.Model(&publicPublication).Association("Tags").Append(&publicTag); err != nil {
		t.Fatalf("associate public tag: %v", err)
	}

	unlistedPost := db.Post{
		Content:    "# 隐藏\n正文",
		Status:     "published",
		UserID:     1,
		Visibility: db.PostVisibilityUnlisted,
	}
	if err := gdb.Create(&unlistedPost).Error; err != nil {
		t.Fatalf("create unlisted post: %v", err)
	}
	unlistedPublication := db.PostPublication{
		PostID:      unlistedPost.ID,
		Content:     unlistedPost.Content,
		UserID:      1,
		PublishedAt: time.Now(),
		Version:     1,
		Visibility:  db.PostVisibilityUnlisted,
	}
	if err := gdb.Create(&unlistedPublication).Error; err != nil {
		t.Fatalf("create unlisted publication: %v", err)
	}
	if err := gdb.Model(&unlistedPost).Update("latest_publication_id", unlistedPublication.ID).Error; err != nil {
		t.Fatalf("update unlisted latest publication: %v", err)
	}
	if err := gdb.Model(&unlistedPublication).Association("Tags").Append(&unlistedTag); err != nil {
		t.Fatalf("associate unlisted tag: %v", err)
	}

	svc := NewTagService(gdb)
	usages, err := svc.PublishedUsage()
	if err != nil {
		t.Fatalf("published usage: %v", err)
	}

	if len(usages) != 1 {
		t.Fatalf("expected only discoverable tags, got %d", len(usages))
	}
	if usages[0].Name != publicTag.Name {
		t.Fatalf("expected discoverable tag %s, got %s", publicTag.Name, usages[0].Name)
	}
}

func TestTagService_PublishedUsageUsesLatestPublicationVisibility(t *testing.T) {
	gdb, cleanup := setupTagServiceTestDB(t)
	defer cleanup()

	tag := db.Tag{Name: "可见标签", SortOrder: 0}
	if err := gdb.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}

	post := db.Post{
		Content:    "# 可见度\n正文",
		Status:     "published",
		UserID:     1,
		Visibility: db.PostVisibilityPublic,
	}
	if err := gdb.Create(&post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	publication := db.PostPublication{
		PostID:      post.ID,
		Content:     post.Content,
		UserID:      1,
		PublishedAt: time.Now(),
		Version:     1,
		Visibility:  db.PostVisibilityPublic,
	}
	if err := gdb.Create(&publication).Error; err != nil {
		t.Fatalf("create publication: %v", err)
	}
	if err := gdb.Model(&post).Update("latest_publication_id", publication.ID).Error; err != nil {
		t.Fatalf("update latest publication id: %v", err)
	}
	if err := gdb.Model(&publication).Association("Tags").Append(&tag); err != nil {
		t.Fatalf("associate tag: %v", err)
	}

	svc := NewTagService(gdb)

	beforeDraftUpdate, err := svc.PublishedUsage()
	if err != nil {
		t.Fatalf("published usage before draft update: %v", err)
	}
	if len(beforeDraftUpdate) != 1 {
		t.Fatalf("expected 1 discoverable tag before draft update, got %d", len(beforeDraftUpdate))
	}

	if err := gdb.Model(&db.Post{}).Where("id = ?", post.ID).Update("visibility", db.PostVisibilityUnlisted).Error; err != nil {
		t.Fatalf("update post visibility for draft change: %v", err)
	}

	afterDraftUpdate, err := svc.PublishedUsage()
	if err != nil {
		t.Fatalf("published usage after draft update: %v", err)
	}
	if len(afterDraftUpdate) != 1 {
		t.Fatalf("expected tag usage to follow latest publication visibility before republish, got %d", len(afterDraftUpdate))
	}

	if err := gdb.Model(&db.PostPublication{}).Where("id = ?", publication.ID).Update("visibility", db.PostVisibilityUnlisted).Error; err != nil {
		t.Fatalf("update publication visibility for republish snapshot: %v", err)
	}

	afterRepublishSnapshot, err := svc.PublishedUsage()
	if err != nil {
		t.Fatalf("published usage after republish snapshot: %v", err)
	}
	if len(afterRepublishSnapshot) != 0 {
		t.Fatalf("expected unlisted latest publication to be excluded, got %d", len(afterRepublishSnapshot))
	}
}
