package db

import "gorm.io/gorm"

// SystemSetting 存储后台可配置的系统级键值对。
type SystemSetting struct {
	gorm.Model
	Key   string `gorm:"size:100;uniqueIndex;not null"`
	Value string `gorm:"type:text"`
}

// TableName 自定义表名以保持命名一致。
func (SystemSetting) TableName() string {
	return "system_settings"
}

const (
	// SettingKeySiteName 表示站点名称。
	SettingKeySiteName = "site_name"
	// SettingKeySiteLogoURL 表示站点 Logo 链接。
	SettingKeySiteLogoURL = "site_logo_url"
	// SettingKeyOpenAIAPIKey 表示 OpenAI API Key。
	SettingKeyOpenAIAPIKey = "openai_api_key"
)
