package service

import (
	"testing"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupProfileServiceTestDB(t *testing.T) func() {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.ProfileContact{}); err != nil {
		t.Fatalf("failed to migrate profile contacts: %v", err)
	}

	db.DB = gdb

	return func() {
		sqlDB, err := db.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestProfileServiceCreateAndList(t *testing.T) {
	cleanup := setupProfileServiceTestDB(t)
	defer cleanup()

	svc := NewProfileService(db.DB)
	if _, err := svc.CreateContact(ProfileContactInput{Platform: "微信", Label: "个人微信", Value: "coder"}); err != nil {
		t.Fatalf("create contact failed: %v", err)
	}
	second, err := svc.CreateContact(ProfileContactInput{Platform: "GitHub", Label: "GitHub", Value: "commitlog", Visible: boolPtr(false)})
	if err != nil {
		t.Fatalf("create contact failed: %v", err)
	}
	if second.Visible {
		t.Fatalf("expected second contact to be hidden")
	}

	visible, err := svc.ListContacts(false)
	if err != nil {
		t.Fatalf("list contacts failed: %v", err)
	}
	if len(visible) != 1 {
		t.Fatalf("expected 1 visible contact, got %d", len(visible))
	}

	all, err := svc.ListContacts(true)
	if err != nil {
		t.Fatalf("list all contacts failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(all))
	}
	if all[0].Sort != 0 || all[1].Sort != 1 {
		t.Fatalf("expected contacts to have incremental sort values, got %d and %d", all[0].Sort, all[1].Sort)
	}
}

func TestProfileServiceUpdate(t *testing.T) {
	cleanup := setupProfileServiceTestDB(t)
	defer cleanup()

	svc := NewProfileService(db.DB)
	created, err := svc.CreateContact(ProfileContactInput{Platform: "邮箱", Label: "工作邮箱", Value: "me@example.com"})
	if err != nil {
		t.Fatalf("create contact failed: %v", err)
	}

	updated, err := svc.UpdateContact(created.ID, ProfileContactInput{Platform: "邮箱", Label: "联系邮箱", Value: "hi@example.com", Icon: "email", Visible: boolPtr(false), Sort: intPtr(5)})
	if err != nil {
		t.Fatalf("update contact failed: %v", err)
	}

	if updated.Label != "联系邮箱" || updated.Value != "hi@example.com" {
		t.Fatalf("update did not persist fields: %#v", updated)
	}
	if updated.Visible {
		t.Fatalf("expected contact to be hidden after update")
	}
	if updated.Sort != 5 {
		t.Fatalf("expected sort to be updated to 5, got %d", updated.Sort)
	}
}

func TestProfileServiceReorder(t *testing.T) {
	cleanup := setupProfileServiceTestDB(t)
	defer cleanup()

	svc := NewProfileService(db.DB)
	c1, _ := svc.CreateContact(ProfileContactInput{Platform: "A", Label: "A", Value: "a"})
	c2, _ := svc.CreateContact(ProfileContactInput{Platform: "B", Label: "B", Value: "b"})
	c3, _ := svc.CreateContact(ProfileContactInput{Platform: "C", Label: "C", Value: "c"})

	if err := svc.ReorderContacts([]uint{c3.ID, c1.ID, c2.ID}); err != nil {
		t.Fatalf("reorder contacts failed: %v", err)
	}

	items, err := svc.ListContacts(true)
	if err != nil {
		t.Fatalf("list contacts failed: %v", err)
	}

	if items[0].ID != c3.ID || items[0].Sort != 0 {
		t.Fatalf("expected contact c3 to be first with sort 0")
	}
	if items[1].ID != c1.ID || items[1].Sort != 1 {
		t.Fatalf("expected contact c1 to be second with sort 1")
	}
	if items[2].ID != c2.ID || items[2].Sort != 2 {
		t.Fatalf("expected contact c2 to be third with sort 2")
	}
}

func TestProfileServiceValidation(t *testing.T) {
	cleanup := setupProfileServiceTestDB(t)
	defer cleanup()

	svc := NewProfileService(db.DB)
	if _, err := svc.CreateContact(ProfileContactInput{}); err == nil {
		t.Fatal("expected validation error for empty input")
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func intPtr(v int) *int {
	return &v
}
