package handler

import (
	"testing"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
)

func TestLocalizePostListTagsInPlace(t *testing.T) {
	t.Parallel()

	tags := []db.Tag{
		{
			Model: gorm.Model{ID: 1},
			Name:  "产品",
			Translations: []db.TagTranslation{
				{Language: "zh", Name: "产品"},
				{Language: "en", Name: "Product"},
			},
		},
		{
			Model: gorm.Model{ID: 2},
			Name:  "工程",
			Translations: []db.TagTranslation{
				{Language: "zh", Name: "工程"},
			},
		},
	}

	posts := []db.Post{
		{
			Model: gorm.Model{ID: 10},
			Tags: []db.Tag{
				{Model: gorm.Model{ID: 1}, Name: "产品"},
				{Model: gorm.Model{ID: 2}, Name: "工程"},
			},
		},
	}

	tagNamesByID := localizeTagSliceInPlace(tags, "en")
	localizePostTagNamesInPlace(posts, tagNamesByID)

	if tags[0].Name != "Product" {
		t.Fatalf("expected tag 1 name to be %q, got %q", "Product", tags[0].Name)
	}
	if tags[1].Name != "工程" {
		t.Fatalf("expected tag 2 name to stay %q, got %q", "工程", tags[1].Name)
	}
	if posts[0].Tags[0].Name != "Product" {
		t.Fatalf("expected post tag 1 name to be %q, got %q", "Product", posts[0].Tags[0].Name)
	}
	if posts[0].Tags[1].Name != "工程" {
		t.Fatalf("expected post tag 2 name to be %q, got %q", "工程", posts[0].Tags[1].Name)
	}
}
