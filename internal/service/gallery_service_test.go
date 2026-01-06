package service

import (
	"testing"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupGalleryTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := gdb.AutoMigrate(&db.GalleryImage{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return gdb, func() {
		sqlDB, err := gdb.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestGalleryCreateAndList(t *testing.T) {
	gdb, cleanup := setupGalleryTestDB(t)
	defer cleanup()

	svc := NewGalleryService(gdb)
	if _, err := svc.Create(GalleryInput{}); err == nil {
		t.Fatalf("expected error for missing image")
	}

	item, err := svc.Create(GalleryInput{
		Title:       "测试作品",
		Description: "作品描述",
		ImageURL:    "https://example.com/image.jpg",
		ImageWidth:  1200,
		ImageHeight: 800,
		Status:      "",
		SortOrder:   0,
	})
	if err != nil {
		t.Fatalf("failed to create gallery image: %v", err)
	}
	if item.Status != GalleryStatusPublished {
		t.Fatalf("expected status to default to published, got %s", item.Status)
	}
	if item.SortOrder == 0 {
		t.Fatalf("expected sort order to be assigned")
	}

	result, err := svc.ListPublished(1, 6)
	if err != nil {
		t.Fatalf("failed to list published images: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
}

func TestGalleryUpdate(t *testing.T) {
	gdb, cleanup := setupGalleryTestDB(t)
	defer cleanup()

	svc := NewGalleryService(gdb)
	item, err := svc.Create(GalleryInput{
		Title:       "初始标题",
		Description: "初始描述",
		ImageURL:    "https://example.com/image.jpg",
		ImageWidth:  1200,
		ImageHeight: 800,
		Status:      GalleryStatusPublished,
		SortOrder:   5,
	})
	if err != nil {
		t.Fatalf("failed to create gallery image: %v", err)
	}

	updated, err := svc.Update(item.ID, GalleryInput{
		Title:       "更新标题",
		Description: "更新描述",
		ImageURL:    "https://example.com/updated.jpg",
		ImageWidth:  1400,
		ImageHeight: 900,
		Status:      GalleryStatusDraft,
		SortOrder:   2,
	})
	if err != nil {
		t.Fatalf("failed to update gallery image: %v", err)
	}
	if updated.Title != "更新标题" || updated.Status != GalleryStatusDraft {
		t.Fatalf("expected updated fields to persist")
	}
}
