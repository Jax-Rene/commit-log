package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTemplateServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:template-service-%d?mode=memory&cache=shared", time.Now().UnixNano())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open template test db: %v", err)
	}

	if err := gdb.AutoMigrate(&db.Tag{}, &db.PostTemplate{}); err != nil {
		t.Fatalf("failed to migrate template test db: %v", err)
	}
	return gdb
}

func TestTemplateServiceCRUDAndList(t *testing.T) {
	gdb := setupTemplateServiceTestDB(t)
	svc := NewTemplateService(gdb)

	tagA := db.Tag{Name: "周报"}
	tagB := db.Tag{Name: "复盘"}
	if err := gdb.Create(&[]db.Tag{tagA, tagB}).Error; err != nil {
		t.Fatalf("seed tags: %v", err)
	}

	var tags []db.Tag
	if err := gdb.Order("id asc").Find(&tags).Error; err != nil {
		t.Fatalf("load tags: %v", err)
	}

	created, err := svc.Create(PostTemplateInput{
		Name:        "每周复盘模板",
		Description: "固定结构模板",
		Content:     "# {{title}}\n日期: {{date}}",
		Summary:     "用于每周总结",
		Visibility:  db.PostVisibilityUnlisted,
		TagIDs:      []uint{tags[0].ID, tags[1].ID},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if created.Name != "每周复盘模板" {
		t.Fatalf("unexpected template name: %s", created.Name)
	}
	if created.Visibility != db.PostVisibilityUnlisted {
		t.Fatalf("expected visibility %s, got %s", db.PostVisibilityUnlisted, created.Visibility)
	}
	if len(created.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(created.Tags))
	}

	list, err := svc.List(TemplateFilter{
		Keyword: "复盘",
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("expected list total 1, got %d", list.Total)
	}
	if len(list.Templates) != 1 || list.Templates[0].ID != created.ID {
		t.Fatalf("expected listed template id %d", created.ID)
	}

	updated, err := svc.Update(created.ID, PostTemplateInput{
		Name:       "每周复盘模板-v2",
		Content:    "# 标题\n正文",
		Summary:    "更新摘要",
		Visibility: "",
		TagIDs:     []uint{tags[0].ID},
	})
	if err != nil {
		t.Fatalf("update template: %v", err)
	}
	if updated.Visibility != db.PostVisibilityUnlisted {
		t.Fatalf("expected visibility fallback %s, got %s", db.PostVisibilityUnlisted, updated.Visibility)
	}
	if len(updated.Tags) != 1 || updated.Tags[0].ID != tags[0].ID {
		t.Fatalf("expected one tag %d after update", tags[0].ID)
	}

	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("delete template: %v", err)
	}
	if _, err := svc.Get(created.ID); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound after delete, got %v", err)
	}
}

func TestTemplateServiceRenderContent(t *testing.T) {
	gdb := setupTemplateServiceTestDB(t)
	svc := NewTemplateService(gdb)

	now := time.Date(2026, 3, 1, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	rendered := svc.RenderContent(
		"# {{title}}\n发布日期 {{date}}\n时间 {{datetime}}\n未知变量 {{unknown}}",
		TemplateRenderInput{
			Title: "周报第 12 期",
			Now:   now,
		},
	)
	if !strings.Contains(rendered, "# 周报第 12 期") {
		t.Fatalf("expected rendered title placeholder, got: %s", rendered)
	}
	if !strings.Contains(rendered, "2026-03-01") {
		t.Fatalf("expected rendered date placeholder, got: %s", rendered)
	}
	if !strings.Contains(rendered, "2026-03-01 09:30") {
		t.Fatalf("expected rendered datetime placeholder, got: %s", rendered)
	}
	if !strings.Contains(rendered, "{{unknown}}") {
		t.Fatalf("expected unknown placeholder remain unchanged, got: %s", rendered)
	}
}
