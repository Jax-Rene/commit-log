package service

import (
	"testing"

	"github.com/commitlog/internal/db"
)

func TestTagService_UpsertTranslation(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	tagSvc := NewTagService(gdb)

	tag := db.Tag{Name: "产品"}
	if err := gdb.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}

	created, err := tagSvc.UpsertTranslation(tag.ID, TagTranslationInput{
		Language: "en",
		Name:     "Product",
	})
	if err != nil {
		t.Fatalf("upsert translation: %v", err)
	}
	if created.TagID != tag.ID || created.Language != "en" || created.Name != "Product" {
		t.Fatalf("unexpected translation: %+v", created)
	}

	updated, err := tagSvc.UpsertTranslation(tag.ID, TagTranslationInput{
		Language: "en",
		Name:     "Product Updated",
	})
	if err != nil {
		t.Fatalf("upsert translation update: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("expected translation to be updated in-place")
	}
	if updated.Name != "Product Updated" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}

	var count int64
	if err := gdb.Model(&db.TagTranslation{}).Where("tag_id = ? AND language = ?", tag.ID, "en").Count(&count).Error; err != nil {
		t.Fatalf("count translations: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 translation row, got %d", count)
	}
}
