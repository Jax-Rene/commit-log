package service

import (
	"testing"
	"time"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupHabitTestDB(t *testing.T) func() {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.Habit{}, &db.HabitLog{}); err != nil {
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

func TestHabitServiceCreateAndList(t *testing.T) {
	cleanup := setupHabitTestDB(t)
	defer cleanup()

	svc := NewHabitService(db.DB)

	habit, err := svc.Create(HabitInput{
		Name:           "晨跑",
		Description:    "每天 5 公里",
		FrequencyUnit:  "daily",
		FrequencyCount: 1,
		TypeTag:        "健康",
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if habit.ID == 0 {
		t.Fatal("expected habit to have ID")
	}

	if habit.Status != "active" {
		t.Fatalf("unexpected status: %s", habit.Status)
	}

	habits, err := svc.List(HabitFilter{Status: "active"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(habits) != 1 {
		t.Fatalf("expected 1 habit, got %d", len(habits))
	}

	// 不合法频率
	if _, err := svc.Create(HabitInput{Name: "阅读", FrequencyUnit: "yearly", FrequencyCount: 1}); err == nil {
		t.Fatal("expected error for invalid frequency unit")
	}
}

func TestHabitServiceUpdate(t *testing.T) {
	cleanup := setupHabitTestDB(t)
	defer cleanup()

	svc := NewHabitService(db.DB)
	habit, err := svc.Create(HabitInput{
		Name:           "冥想",
		FrequencyUnit:  "daily",
		FrequencyCount: 1,
	})
	if err != nil {
		t.Fatalf("failed to create habit: %v", err)
	}

	updated, err := svc.Update(habit.ID, HabitInput{
		Name:           "冥想训练",
		Description:    "晚间 10 分钟",
		FrequencyUnit:  "weekly",
		FrequencyCount: 3,
		Status:         "inactive",
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	if updated.Name != "冥想训练" {
		t.Fatalf("expected name to update, got %s", updated.Name)
	}

	if updated.Status != "inactive" {
		t.Fatalf("expected status inactive, got %s", updated.Status)
	}
}

func TestHabitLogUpsertAndStats(t *testing.T) {
	cleanup := setupHabitTestDB(t)
	defer cleanup()

	habitSvc := NewHabitService(db.DB)
	habit, err := habitSvc.Create(HabitInput{
		Name:           "写日记",
		FrequencyUnit:  "daily",
		FrequencyCount: 1,
	})
	if err != nil {
		t.Fatalf("failed to create habit: %v", err)
	}

	logSvc := NewHabitLogService(db.DB)
	base := time.Date(2024, 5, 1, 0, 0, 0, 0, time.Local)

	for i := 0; i < 3; i++ {
		date := base.AddDate(0, 0, i)
		if _, err := logSvc.Upsert(HabitLogInput{HabitID: habit.ID, LogDate: date, Note: "完成"}); err != nil {
			t.Fatalf("Upsert returned error: %v", err)
		}
	}

	// 重复日期更新备注
	if _, err := logSvc.Upsert(HabitLogInput{HabitID: habit.ID, LogDate: base, Note: "补记"}); err != nil {
		t.Fatalf("Upsert update returned error: %v", err)
	}

	logs, err := logSvc.ListBetween(HabitLogFilter{HabitID: habit.ID, Start: base, End: base.AddDate(0, 0, 2)})
	if err != nil {
		t.Fatalf("ListBetween returned error: %v", err)
	}

	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}

	if logs[0].Note != "补记" {
		t.Fatalf("expected note to update, got %s", logs[0].Note)
	}

	stats, err := logSvc.StatsBetween(HabitLogFilter{HabitID: habit.ID, Start: base, End: base.AddDate(0, 0, 2)}, *habit)
	if err != nil {
		t.Fatalf("StatsBetween returned error: %v", err)
	}

	if stats.CompletedCount != 3 {
		t.Fatalf("expected completed count 3, got %d", stats.CompletedCount)
	}

	if stats.TargetCount != 3 {
		t.Fatalf("expected target count 3, got %d", stats.TargetCount)
	}

	if stats.CurrentStreak != 3 || stats.LongestStreak != 3 {
		t.Fatalf("unexpected streaks: current=%d longest=%d", stats.CurrentStreak, stats.LongestStreak)
	}
}
