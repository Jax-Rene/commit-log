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
	// SettingKeySiteDescription 表示站点默认描述文案。
	SettingKeySiteDescription = "site_description"
	// SettingKeySiteKeywords 表示站点默认关键词列表。
	SettingKeySiteKeywords = "site_keywords"
	// SettingKeySiteSocialImage 表示站点社交分享默认配图。
	SettingKeySiteSocialImage = "site_social_image"
	// SettingKeyOpenAIAPIKey 表示 OpenAI API Key。
	SettingKeyOpenAIAPIKey = "openai_api_key"
	// SettingKeyAIProvider 表示当前使用的 AI 平台。
	SettingKeyAIProvider = "ai_provider"
	// SettingKeyDeepSeekAPIKey 表示 DeepSeek API Key。
	SettingKeyDeepSeekAPIKey = "deepseek_api_key"
	// SettingKeyAISummaryPrompt 表示摘要生成的系统提示语。
	SettingKeyAISummaryPrompt = "ai_summary_prompt"
	// SettingKeyAIRewritePrompt 表示全文优化的系统提示语。
	SettingKeyAIRewritePrompt = "ai_rewrite_prompt"
	// SettingKeySiteAdminFooter 表示后台页脚文案。
	SettingKeySiteAdminFooter = "site_admin_footer"
	// SettingKeySitePublicFooter 表示前台页脚文案。
	SettingKeySitePublicFooter = "site_public_footer"
	// SettingKeyGalleryEnabled 表示是否启用 Gallery。
	SettingKeyGalleryEnabled = "gallery_enabled"
)
