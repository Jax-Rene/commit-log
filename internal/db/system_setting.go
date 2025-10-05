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
        // SettingKeySiteLogoURLLight 表示浅色模式下的站点 Logo 链接。
        SettingKeySiteLogoURLLight = "site_logo_url_light"
        // SettingKeySiteLogoURLDark 表示深色模式下的站点 Logo 链接。
        SettingKeySiteLogoURLDark = "site_logo_url_dark"
        // SettingKeyOpenAIAPIKey 表示 OpenAI API Key。
        SettingKeyOpenAIAPIKey = "openai_api_key"
        // SettingKeyAIProvider 表示当前使用的 AI 平台。
        SettingKeyAIProvider = "ai_provider"
        // SettingKeyDeepSeekAPIKey 表示 DeepSeek API Key。
        SettingKeyDeepSeekAPIKey = "deepseek_api_key"
        // SettingKeySiteAdminFooter 表示后台页脚文案。
        SettingKeySiteAdminFooter = "site_admin_footer"
        // SettingKeySitePublicFooter 表示前台页脚文案。
        SettingKeySitePublicFooter = "site_public_footer"
)
