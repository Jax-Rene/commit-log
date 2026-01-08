package main

import (
	"testing"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	expectedGallerySeedCount     = 19
	minPublishedGallerySeedCount = 13
)

func setupGallerySeedTestDB(t *testing.T) func() {
	t.Helper()

	gdb, err := gorm.Open(sqlite.Open("file:gallery-seed?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := gdb.AutoMigrate(&db.GalleryImage{}); err != nil {
		t.Fatalf("failed to migrate gallery images: %v", err)
	}

	db.DB = gdb

	return func() {
		sqlDB, err := gdb.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestCreateTestGalleryImagesSeedsVariation(t *testing.T) {
	cleanup := setupGallerySeedTestDB(t)
	defer cleanup()

	if err := db.DB.Create(&db.GalleryImage{
		Title:       "legacy record",
		Description: "seeded for deletion check",
		ImageURL:    "https://example.com/old.jpg",
		ImageWidth:  1200,
		ImageHeight: 800,
		Status:      "published",
	}).Error; err != nil {
		t.Fatalf("failed to seed pre-existing image: %v", err)
	}

	createTestGalleryImages()

	var items []db.GalleryImage
	if err := db.DB.Find(&items).Error; err != nil {
		t.Fatalf("failed to list gallery images: %v", err)
	}
	if len(items) != expectedGallerySeedCount {
		t.Fatalf("expected %d gallery images, got %d", expectedGallerySeedCount, len(items))
	}

	published := 0
	hasLandscape := false
	hasPortrait := false
	hasSquare := false

	for _, item := range items {
		if item.ImageWidth <= 0 || item.ImageHeight <= 0 {
			t.Fatalf("expected image dimensions to be set for item %d", item.ID)
		}
		if item.Status == "published" {
			published++
		}
		ratio := float64(item.ImageWidth) / float64(item.ImageHeight)
		switch {
		case ratio > 1.15:
			hasLandscape = true
		case ratio < 0.9:
			hasPortrait = true
		default:
			hasSquare = true
		}
	}

	if published < minPublishedGallerySeedCount {
		t.Fatalf("expected at least %d published items, got %d", minPublishedGallerySeedCount, published)
	}
	if !hasLandscape || !hasPortrait || !hasSquare {
		t.Fatalf("expected landscape, portrait, and square aspect ratios to exist")
	}
}
