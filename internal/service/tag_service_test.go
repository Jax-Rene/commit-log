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

	if err := gdb.AutoMigrate(&db.Tag{}, &db.Post{}); err != nil {
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
