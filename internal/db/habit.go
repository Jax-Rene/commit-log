package db

import (
	"time"

	"gorm.io/gorm"
)

// Habit 定义了习惯模型
// 频率通过 FrequencyUnit/FrequencyCount 描述，例如 unit=daily/count=1
// TypeTag 用于区分习惯类别，便于统计/筛选
// Status 预留 active/inactive 控制后台展示
// StartDate/EndDate 便于未来扩展有效期，暂未强制使用
// JSON 序列化字段后续用于 API
// NOTE: 保持结构精简，更多配置可迭代扩展
type Habit struct {
	gorm.Model
	Name           string
	Description    string
	FrequencyUnit  string
	FrequencyCount int
	TypeTag        string
	Status         string
	StartDate      *time.Time
	EndDate        *time.Time
}

// HabitLog 记录习惯打卡日志
// Habit + LogDate 采用唯一索引，保证幂等；LogTime 存储用户选择的具体时间
// Source 标记打卡来源（manual/admin 自动等），Note 为备注
type HabitLog struct {
	gorm.Model
	HabitID uint      `gorm:"index;index:idx_habit_log_unique,unique"`
	Habit   Habit     `gorm:"constraint:OnDelete:CASCADE"`
	LogDate time.Time `gorm:"index:idx_habit_log_unique,unique"`
	LogTime *time.Time
	Source  string
	Note    string
}

// TableName 重写确保唯一索引作用到 habit_id + log_date
func (HabitLog) TableName() string {
	return "habit_logs"
}
