package service

import (
	"testing"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupPageServiceTestDB(t *testing.T) func() {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.Page{}, &db.ProfileContact{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	db.DB = gdb

	return func() {
		sqlDB, err := db.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestSaveAboutPageCreatesRecord(t *testing.T) {
	cleanup := setupPageServiceTestDB(t)
	defer cleanup()

	svc := NewPageService(db.DB)
	page, err := svc.SaveAboutPage("# Hello\n这是关于页", "zh")
	if err != nil {
		t.Fatalf("SaveAboutPage returned error: %v", err)
	}

	if page.Slug != "about" {
		t.Fatalf("expected slug 'about', got %s", page.Slug)
	}

	if page.Title != "关于我" {
		t.Fatalf("expected default title, got %s", page.Title)
	}

	if page.Content == "" {
		t.Fatal("expected content to be persisted")
	}
	if page.Language != "zh" {
		t.Fatalf("expected language zh, got %s", page.Language)
	}
}

func TestSaveAboutPageUpdatesExisting(t *testing.T) {
	cleanup := setupPageServiceTestDB(t)
	defer cleanup()

	svc := NewPageService(db.DB)
	if _, err := svc.SaveAboutPage("初始内容", "zh"); err != nil {
		t.Fatalf("failed to seed about page: %v", err)
	}

	updated, err := svc.SaveAboutPage("更新后的内容", "zh")
	if err != nil {
		t.Fatalf("SaveAboutPage returned error: %v", err)
	}

	if updated.Content != "更新后的内容" {
		t.Fatalf("expected content to be updated, got %s", updated.Content)
	}
}

func TestSaveAboutPageRejectsEmptyContent(t *testing.T) {
	cleanup := setupPageServiceTestDB(t)
	defer cleanup()

	svc := NewPageService(db.DB)
	if _, err := svc.SaveAboutPage("\n\t ", "zh"); err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestSaveAboutPageSeparatesLanguages(t *testing.T) {
	cleanup := setupPageServiceTestDB(t)
	defer cleanup()

	svc := NewPageService(db.DB)
	zhPage, err := svc.SaveAboutPage("中文内容", "zh")
	if err != nil {
		t.Fatalf("save zh page: %v", err)
	}
	enPage, err := svc.SaveAboutPage("English Content", "en")
	if err != nil {
		t.Fatalf("save en page: %v", err)
	}

	if zhPage.Language == enPage.Language {
		t.Fatalf("expected different languages, got %s", zhPage.Language)
	}
	if zhPage.Slug != "about" || enPage.Slug != "about" {
		t.Fatalf("expected both pages to share slug about")
	}
}
