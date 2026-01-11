package service

import (
	"testing"

	"github.com/commitlog/internal/db"
)

func TestTagService_BackfillTagTranslations(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	tagSvc := NewTagService(gdb)

	tags := []db.Tag{
		{Name: "产品"},
		{Name: "Go"},
	}
	if err := gdb.Create(&tags).Error; err != nil {
		t.Fatalf("create tags: %v", err)
	}

	// Seed one existing translation to ensure backfill is idempotent.
	seed := db.TagTranslation{TagID: tags[0].ID, Language: "zh", Name: "产品"}
	if err := gdb.Create(&seed).Error; err != nil {
		t.Fatalf("seed translation: %v", err)
	}

	created, err := tagSvc.BackfillTagTranslations([]string{"zh", "en"})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	// tags[0]: missing en => 1
	// tags[1]: missing zh+en => 2
	if created != 3 {
		t.Fatalf("expected 3 created translations, got %d", created)
	}

	createdAgain, err := tagSvc.BackfillTagTranslations([]string{"zh", "en"})
	if err != nil {
		t.Fatalf("backfill again: %v", err)
	}
	if createdAgain != 0 {
		t.Fatalf("expected idempotent backfill, got %d", createdAgain)
	}
}
