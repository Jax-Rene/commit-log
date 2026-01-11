package service

import (
	"testing"

	"github.com/commitlog/internal/db"
)

func TestTagService_PublishedUsageFiltersByLanguage(t *testing.T) {
	gdb := setupPostServiceTestDB(t)
	postSvc := NewPostService(gdb)
	tagSvc := NewTagService(gdb)

	user := db.User{Username: "tag-lang"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	tag := db.Tag{Name: "Go"}
	if err := gdb.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}

	createPost := func(language string) {
		post, err := postSvc.Create(PostInput{
			Content:     "# 标题\n内容",
			Summary:     "摘要",
			UserID:      user.ID,
			TagIDs:      []uint{tag.ID},
			Language:    language,
			CoverURL:    "https://example.com/cover.jpg",
			CoverWidth:  1200,
			CoverHeight: 800,
		})
		if err != nil {
			t.Fatalf("create post: %v", err)
		}
		if _, err := postSvc.Publish(post.ID, user.ID, nil); err != nil {
			t.Fatalf("publish post: %v", err)
		}
	}

	createPost("zh")
	createPost("en")

	zhUsage, err := tagSvc.PublishedUsage("zh")
	if err != nil {
		t.Fatalf("published usage zh: %v", err)
	}
	if len(zhUsage) != 1 || zhUsage[0].Count != 1 {
		t.Fatalf("expected zh usage count 1, got %+v", zhUsage)
	}

	enUsage, err := tagSvc.PublishedUsage("en")
	if err != nil {
		t.Fatalf("published usage en: %v", err)
	}
	if len(enUsage) != 1 || enUsage[0].Count != 1 {
		t.Fatalf("expected en usage count 1, got %+v", enUsage)
	}
}
