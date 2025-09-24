package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrHabitNotFound 在指定习惯不存在时返回
	ErrHabitNotFound = errors.New("habit not found")
	// ErrHabitInvalidFrequency 当频率配置异常时返回
	ErrHabitInvalidFrequency = errors.New("invalid habit frequency configuration")
)

// HabitService 负责 Habit 数据的增删改查
// 主要用于后台管理逻辑，保持与 handler 解耦
// FrequencyUnit 支持 daily/weekly/monthly，FrequencyCount>0
// Status 仅使用 active/inactive，默认 active

type HabitService struct {
	db *gorm.DB
}

// HabitFilter 描述后台列表过滤条件
type HabitFilter struct {
	Status  string
	TypeTag string
	Search  string
}

// HabitInput 定义创建/更新习惯时可配置字段
type HabitInput struct {
	Name           string
	Description    string
	FrequencyUnit  string
	FrequencyCount int
	TypeTag        string
	Status         string
	StartDate      *time.Time
	EndDate        *time.Time
}

// NewHabitService 构造 HabitService
func NewHabitService(gdb *gorm.DB) *HabitService {
	return &HabitService{db: gdb}
}

// List 返回习惯集合，支持基本筛选
func (s *HabitService) List(filter HabitFilter) ([]db.Habit, error) {
	var habits []db.Habit

	query := s.db.Model(&db.Habit{})

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.TypeTag != "" {
		query = query.Where("type_tag = ?", filter.TypeTag)
	}
	if filter.Search != "" {
		like := fmt.Sprintf("%%%s%%", strings.TrimSpace(filter.Search))
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}

	if err := query.Order("created_at DESC").Find(&habits).Error; err != nil {
		return nil, fmt.Errorf("list habits: %w", err)
	}

	return habits, nil
}

// Get 根据 ID 获取习惯
func (s *HabitService) Get(id uint) (*db.Habit, error) {
	var habit db.Habit
	if err := s.db.First(&habit, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHabitNotFound
		}
		return nil, fmt.Errorf("get habit: %w", err)
	}
	return &habit, nil
}

// Create 新建习惯
func (s *HabitService) Create(input HabitInput) (*db.Habit, error) {
	if err := validateHabitInput(input); err != nil {
		return nil, err
	}

	habit := db.Habit{
		Name:           strings.TrimSpace(input.Name),
		Description:    strings.TrimSpace(input.Description),
		FrequencyUnit:  strings.TrimSpace(input.FrequencyUnit),
		FrequencyCount: input.FrequencyCount,
		TypeTag:        strings.TrimSpace(input.TypeTag),
		Status:         normalizeStatus(input.Status),
		StartDate:      input.StartDate,
		EndDate:        input.EndDate,
	}

	if err := s.db.Create(&habit).Error; err != nil {
		return nil, fmt.Errorf("create habit: %w", err)
	}
	return &habit, nil
}

// Update 更新习惯
func (s *HabitService) Update(id uint, input HabitInput) (*db.Habit, error) {
	if err := validateHabitInput(input); err != nil {
		return nil, err
	}

	var existing db.Habit
	if err := s.db.First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHabitNotFound
		}
		return nil, fmt.Errorf("find habit: %w", err)
	}

	existing.Name = strings.TrimSpace(input.Name)
	existing.Description = strings.TrimSpace(input.Description)
	existing.FrequencyUnit = strings.TrimSpace(input.FrequencyUnit)
	existing.FrequencyCount = input.FrequencyCount
	existing.TypeTag = strings.TrimSpace(input.TypeTag)
	existing.Status = normalizeStatus(input.Status)
	existing.StartDate = input.StartDate
	existing.EndDate = input.EndDate

	if err := s.db.Save(&existing).Error; err != nil {
		return nil, fmt.Errorf("update habit: %w", err)
	}
	return &existing, nil
}

// Delete 删除习惯
func (s *HabitService) Delete(id uint) error {
	if err := s.db.Delete(&db.Habit{}, id).Error; err != nil {
		return fmt.Errorf("delete habit: %w", err)
	}
	return nil
}

func validateHabitInput(input HabitInput) error {
	unit := strings.TrimSpace(strings.ToLower(input.FrequencyUnit))
	if unit != "daily" && unit != "weekly" && unit != "monthly" {
		return fmt.Errorf("%w: unsupported unit %s", ErrHabitInvalidFrequency, input.FrequencyUnit)
	}

	if input.FrequencyCount <= 0 {
		return fmt.Errorf("%w: count must be positive", ErrHabitInvalidFrequency)
	}

	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("habit name is required")
	}

	return nil
}

func normalizeStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if status != "inactive" {
		return "active"
	}
	return "inactive"
}

// HabitLogService 负责打卡与统计逻辑
type HabitLogService struct {
	db *gorm.DB
}

// HabitHeatmapEntry 表示热力图中的单日打卡数据
type HabitHeatmapEntry struct {
	LogDate   time.Time
	HabitID   uint
	HabitName string
	HabitType string
}

// HabitLogInput 定义打卡时的输入对象
type HabitLogInput struct {
	HabitID uint
	LogDate time.Time
	LogTime *time.Time
	Source  string
	Note    string
}

// HabitLogFilter 指定查询区间
type HabitLogFilter struct {
	HabitID uint
	Start   time.Time
	End     time.Time
}

// HabitStats 汇总基础统计数据
type HabitStats struct {
	RangeStart     time.Time
	RangeEnd       time.Time
	CompletedCount int
	TargetCount    int
	CompletionRate float64
	CurrentStreak  int
	LongestStreak  int
}

// NewHabitLogService 构造 HabitLogService
func NewHabitLogService(gdb *gorm.DB) *HabitLogService {
	return &HabitLogService{db: gdb}
}

// Upsert 处理幂等打卡逻辑：若存在则更新备注/时间/来源，否则创建
func (s *HabitLogService) Upsert(input HabitLogInput) (*db.HabitLog, error) {
	logDate := normalizeToDate(input.LogDate)

	record := db.HabitLog{
		HabitID: input.HabitID,
		LogDate: logDate,
		Note:    strings.TrimSpace(input.Note),
		Source:  strings.TrimSpace(input.Source),
		LogTime: input.LogTime,
	}

	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "habit_id"}, {Name: "log_date"}},
		DoUpdates: clause.AssignmentColumns([]string{"note", "source", "log_time", "updated_at"}),
	}).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("upsert habit log: %w", err)
	}

	if err := s.db.Where("habit_id = ? AND log_date = ?", input.HabitID, logDate).First(&record).Error; err != nil {
		return nil, fmt.Errorf("reload habit log: %w", err)
	}

	return &record, nil
}

// Delete 删除指定打卡记录
func (s *HabitLogService) Delete(id uint) error {
	if err := s.db.Delete(&db.HabitLog{}, id).Error; err != nil {
		return fmt.Errorf("delete habit log: %w", err)
	}
	return nil
}

// ListBetween 返回指定区间内的打卡记录
func (s *HabitLogService) ListBetween(filter HabitLogFilter) ([]db.HabitLog, error) {
	var logs []db.HabitLog

	if filter.HabitID == 0 {
		return nil, fmt.Errorf("habit id is required")
	}

	start := normalizeToDate(filter.Start)
	end := normalizeToDate(filter.End)

	if err := s.db.Where("habit_id = ?", filter.HabitID).
		Where("log_date BETWEEN ? AND ?", start, end).
		Order("log_date ASC").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("list habit logs: %w", err)
	}

	return logs, nil
}

// HeatmapRange 返回指定区间内所有习惯的打卡数据
func (s *HabitLogService) HeatmapRange(start, end time.Time) ([]HabitHeatmapEntry, error) {
	if end.Before(start) {
		return nil, fmt.Errorf("invalid range: end before start")
	}

	normalizedStart := normalizeToDate(start)
	normalizedEnd := normalizeToDate(end)

	var rows []HabitHeatmapEntry
	if err := s.db.Model(&db.HabitLog{}).
		Select("habit_logs.log_date AS log_date, habit_logs.habit_id AS habit_id, habits.name AS habit_name, habits.type_tag AS habit_type").
		Joins("JOIN habits ON habits.id = habit_logs.habit_id").
		Where("habit_logs.log_date BETWEEN ? AND ?", normalizedStart, normalizedEnd).
		Order("habit_logs.log_date ASC, habits.name ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list heatmap logs: %w", err)
	}

	return rows, nil
}

// StatsBetween 计算区间内的完成数、目标完成数及连胜
func (s *HabitLogService) StatsBetween(filter HabitLogFilter, habit db.Habit) (*HabitStats, error) {
	logs, err := s.ListBetween(filter)
	if err != nil {
		return nil, err
	}

	stats := &HabitStats{
		RangeStart: filter.Start,
		RangeEnd:   filter.End,
	}

	stats.CompletedCount = len(logs)
	stats.TargetCount = expectedCount(habit, filter.Start, filter.End)
	if stats.TargetCount <= 0 {
		stats.TargetCount = stats.CompletedCount
	}

	if stats.TargetCount > 0 {
		stats.CompletionRate = float64(stats.CompletedCount) / float64(stats.TargetCount)
	}

	stats.CurrentStreak, stats.LongestStreak = calculateStreaks(logs)

	return stats, nil
}

func normalizeToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func expectedCount(habit db.Habit, start, end time.Time) int {
	if end.Before(start) {
		return 0
	}

	days := int(end.Sub(start).Hours()/24) + 1

	switch strings.ToLower(habit.FrequencyUnit) {
	case "daily":
		return days * max(1, habit.FrequencyCount)
	case "weekly":
		weeks := int(float64(days) / 7.0)
		if weeks == 0 {
			weeks = 1
		}
		return weeks * max(1, habit.FrequencyCount)
	case "monthly":
		months := diffMonths(start, end)
		if months == 0 {
			months = 1
		}
		return months * max(1, habit.FrequencyCount)
	default:
		return days * max(1, habit.FrequencyCount)
	}
}

func calculateStreaks(logs []db.HabitLog) (current, longest int) {
	if len(logs) == 0 {
		return 0, 0
	}

	longest = 1
	current = 1

	for i := 1; i < len(logs); i++ {
		delta := int(logs[i].LogDate.Sub(logs[i-1].LogDate).Hours() / 24)
		if delta == 1 {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 1
		}
	}

	return current, longest
}

func diffMonths(start, end time.Time) int {
	y1, m1, _ := start.Date()
	y2, m2, _ := end.Date()

	return (int(y2)-int(y1))*12 + int(m2-m1) + 1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
