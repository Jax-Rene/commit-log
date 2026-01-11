package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/locale"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// AIProviderOpenAI 表示使用 OpenAI 能力。
	AIProviderOpenAI = "openai"
	// AIProviderDeepSeek 表示使用 DeepSeek 能力。
	AIProviderDeepSeek = "deepseek"
)

var supportedAIProviders = []string{AIProviderOpenAI, AIProviderDeepSeek}

const (
	defaultSiteName        = "CommitLog"
	defaultSiteDescription = "AI 全栈工程师的技术与成长记录"
	defaultAdminFooter     = "日拱一卒，功不唐捐"
	defaultAdminFooterEn   = "Grow a little every day"
	defaultPublicFooter    = "激发创造，延迟满足"
	defaultPublicFooterEn  = "Create with purpose"
	defaultGalleryEnabled  = true
	defaultGallerySubtitle = "Shot by Lumix S5M2 / OnePlus 13"
	defaultPreferredLang   = locale.LanguageChinese
)

// SystemSettings 描述后台可配置的系统信息。
type SystemSettings struct {
	SiteName           string
	SiteNameZh         string
	SiteNameEn         string
	SiteLogoURL        string
	SiteLogoURLLight   string
	SiteLogoURLDark    string
	SiteDescription    string
	SiteDescriptionZh  string
	SiteDescriptionEn  string
	SiteSocialImage    string
	PreferredLanguage  string
	AIProvider         string
	OpenAIAPIKey       string
	DeepSeekAPIKey     string
	AdminFooterText    string
	AdminFooterTextZh  string
	AdminFooterTextEn  string
	PublicFooterText   string
	PublicFooterTextZh string
	PublicFooterTextEn string
	AISummaryPrompt    string
	AIRewritePrompt    string
	GallerySubtitle    string
	GallerySubtitleZh  string
	GallerySubtitleEn  string
	GalleryEnabled     bool
}

// ErrAIAPIKeyMissing 表示未提供必需的 AI 平台 API Key。
var ErrAIAPIKeyMissing = errors.New("api key is required")

// ErrOpenAIAPIKeyMissing 为历史兼容，等价于 ErrAIAPIKeyMissing。
var ErrOpenAIAPIKeyMissing = ErrAIAPIKeyMissing

// SystemSettingsInput 用于更新系统设置。
type SystemSettingsInput struct {
	SiteName           string
	SiteNameZh         string
	SiteNameEn         string
	SiteLogoURL        string
	SiteLogoURLLight   string
	SiteLogoURLDark    string
	SiteDescription    string
	SiteDescriptionZh  string
	SiteDescriptionEn  string
	SiteSocialImage    string
	PreferredLanguage  string
	AIProvider         string
	OpenAIAPIKey       string
	DeepSeekAPIKey     string
	AdminFooterText    string
	AdminFooterTextZh  string
	AdminFooterTextEn  string
	PublicFooterText   string
	PublicFooterTextZh string
	PublicFooterTextEn string
	AISummaryPrompt    string
	AIRewritePrompt    string
	GallerySubtitle    string
	GallerySubtitleZh  string
	GallerySubtitleEn  string
	GalleryEnabled     *bool
}

// SystemSettingService 提供系统设置的读取与更新能力。
type SystemSettingService struct {
	db              *gorm.DB
	httpClient      httpDoer
	openAIBaseURL   string
	deepSeekBaseURL string
}

// NewSystemSettingService 构造 SystemSettingService。
func NewSystemSettingService(gdb *gorm.DB) *SystemSettingService {
	return &SystemSettingService{
		db:              gdb,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		openAIBaseURL:   "https://api.openai.com/v1",
		deepSeekBaseURL: "https://api.deepseek.com/v1",
	}
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

var settingKeys = []string{
	db.SettingKeySiteName,
	db.SettingKeySiteNameZh,
	db.SettingKeySiteNameEn,
	db.SettingKeySiteLogoURL,
	db.SettingKeySiteLogoURLLight,
	db.SettingKeySiteLogoURLDark,
	db.SettingKeySiteDescription,
	db.SettingKeySiteDescriptionZh,
	db.SettingKeySiteDescriptionEn,
	db.SettingKeySiteSocialImage,
	db.SettingKeySiteAdminFooter,
	db.SettingKeySiteAdminFooterZh,
	db.SettingKeySiteAdminFooterEn,
	db.SettingKeySitePublicFooter,
	db.SettingKeySitePublicFooterZh,
	db.SettingKeySitePublicFooterEn,
	db.SettingKeyGallerySubtitle,
	db.SettingKeyGallerySubtitleZh,
	db.SettingKeyGallerySubtitleEn,
	db.SettingKeyPreferredLanguage,
	db.SettingKeyAIProvider,
	db.SettingKeyOpenAIAPIKey,
	db.SettingKeyDeepSeekAPIKey,
	db.SettingKeyAISummaryPrompt,
	db.SettingKeyAIRewritePrompt,
	db.SettingKeyGalleryEnabled,
}

// GetSettings 读取系统设置，如未设置将返回默认值。
func (s *SystemSettingService) GetSettings() (SystemSettings, error) {
	result := SystemSettings{
		SiteName:           defaultSiteName,
		SiteNameZh:         defaultSiteName,
		SiteNameEn:         defaultSiteName,
		SiteDescription:    defaultSiteDescription,
		SiteDescriptionZh:  defaultSiteDescription,
		SiteDescriptionEn:  defaultSiteDescription,
		PreferredLanguage:  defaultPreferredLang,
		AIProvider:         AIProviderOpenAI,
		AdminFooterText:    defaultAdminFooter,
		AdminFooterTextZh:  defaultAdminFooter,
		AdminFooterTextEn:  defaultAdminFooterEn,
		PublicFooterText:   defaultPublicFooter,
		PublicFooterTextZh: defaultPublicFooter,
		PublicFooterTextEn: defaultPublicFooterEn,
		AISummaryPrompt:    defaultSummarySystemPrompt,
		AIRewritePrompt:    defaultRewriteSystemPrompt,
		GallerySubtitle:    defaultGallerySubtitle,
		GallerySubtitleZh:  defaultGallerySubtitle,
		GallerySubtitleEn:  defaultGallerySubtitle,
		GalleryEnabled:     defaultGalleryEnabled,
	}

	if s == nil || s.db == nil {
		return result, nil
	}

	var records []db.SystemSetting
	if err := s.db.Where("key IN ?", settingKeys).Find(&records).Error; err != nil {
		return result, fmt.Errorf("load system settings: %w", err)
	}

	for _, record := range records {
		switch record.Key {
		case db.SettingKeySiteName:
			if strings.TrimSpace(record.Value) != "" {
				result.SiteName = record.Value
			}
		case db.SettingKeySiteNameZh:
			if strings.TrimSpace(record.Value) != "" {
				result.SiteNameZh = record.Value
			}
		case db.SettingKeySiteNameEn:
			if strings.TrimSpace(record.Value) != "" {
				result.SiteNameEn = record.Value
			}
		case db.SettingKeySiteLogoURL:
			result.SiteLogoURL = record.Value
		case db.SettingKeySiteLogoURLLight:
			result.SiteLogoURLLight = record.Value
		case db.SettingKeySiteLogoURLDark:
			result.SiteLogoURLDark = record.Value
		case db.SettingKeySiteDescription:
			if trimmed := strings.TrimSpace(record.Value); trimmed != "" {
				result.SiteDescription = trimmed
			}
		case db.SettingKeySiteDescriptionZh:
			if trimmed := strings.TrimSpace(record.Value); trimmed != "" {
				result.SiteDescriptionZh = trimmed
			}
		case db.SettingKeySiteDescriptionEn:
			if trimmed := strings.TrimSpace(record.Value); trimmed != "" {
				result.SiteDescriptionEn = trimmed
			}
		case db.SettingKeySiteSocialImage:
			result.SiteSocialImage = strings.TrimSpace(record.Value)
		case db.SettingKeyPreferredLanguage:
			if preferred := locale.NormalizeLanguage(record.Value); preferred != "" {
				result.PreferredLanguage = preferred
			}
		case db.SettingKeySiteAdminFooter:
			if strings.TrimSpace(record.Value) != "" {
				result.AdminFooterText = record.Value
			}
		case db.SettingKeySiteAdminFooterZh:
			if strings.TrimSpace(record.Value) != "" {
				result.AdminFooterTextZh = record.Value
			}
		case db.SettingKeySiteAdminFooterEn:
			if strings.TrimSpace(record.Value) != "" {
				result.AdminFooterTextEn = record.Value
			}
		case db.SettingKeySitePublicFooter:
			if strings.TrimSpace(record.Value) != "" {
				result.PublicFooterText = record.Value
			}
		case db.SettingKeySitePublicFooterZh:
			if strings.TrimSpace(record.Value) != "" {
				result.PublicFooterTextZh = record.Value
			}
		case db.SettingKeySitePublicFooterEn:
			if strings.TrimSpace(record.Value) != "" {
				result.PublicFooterTextEn = record.Value
			}
		case db.SettingKeyGallerySubtitle:
			if trimmed := strings.TrimSpace(record.Value); trimmed != "" {
				result.GallerySubtitle = trimmed
			}
		case db.SettingKeyGallerySubtitleZh:
			if trimmed := strings.TrimSpace(record.Value); trimmed != "" {
				result.GallerySubtitleZh = trimmed
			}
		case db.SettingKeyGallerySubtitleEn:
			if trimmed := strings.TrimSpace(record.Value); trimmed != "" {
				result.GallerySubtitleEn = trimmed
			}
		case db.SettingKeyAIProvider:
			if provider := normalizeAIProvider(record.Value); provider != "" {
				result.AIProvider = provider
			}
		case db.SettingKeyOpenAIAPIKey:
			result.OpenAIAPIKey = record.Value
		case db.SettingKeyDeepSeekAPIKey:
			result.DeepSeekAPIKey = record.Value
		case db.SettingKeyAISummaryPrompt:
			if trimmed := strings.TrimSpace(record.Value); trimmed != "" {
				result.AISummaryPrompt = trimmed
			}
		case db.SettingKeyAIRewritePrompt:
			if trimmed := strings.TrimSpace(record.Value); trimmed != "" {
				result.AIRewritePrompt = trimmed
			}
		case db.SettingKeyGalleryEnabled:
			if parsed, err := strconv.ParseBool(strings.TrimSpace(record.Value)); err == nil {
				result.GalleryEnabled = parsed
			}
		}
	}

	if strings.TrimSpace(result.SiteLogoURLLight) == "" {
		result.SiteLogoURLLight = strings.TrimSpace(result.SiteLogoURL)
	}
	if strings.TrimSpace(result.SiteLogoURLDark) == "" {
		result.SiteLogoURLDark = strings.TrimSpace(result.SiteLogoURL)
	}
	if strings.TrimSpace(result.SiteLogoURL) == "" {
		result.SiteLogoURL = result.SiteLogoURLLight
	}
	if strings.TrimSpace(result.SiteLogoURLLight) == "" {
		result.SiteLogoURLLight = result.SiteLogoURLDark
	}
	if strings.TrimSpace(result.SiteLogoURLDark) == "" {
		result.SiteLogoURLDark = result.SiteLogoURLLight
	}
	if strings.TrimSpace(result.SiteDescription) == "" {
		result.SiteDescription = defaultSiteDescription
	}
	if strings.TrimSpace(result.SiteDescriptionZh) == "" {
		result.SiteDescriptionZh = result.SiteDescription
	}
	if strings.TrimSpace(result.SiteDescriptionEn) == "" {
		result.SiteDescriptionEn = result.SiteDescription
	}
	if strings.TrimSpace(result.SiteName) == "" {
		result.SiteName = defaultSiteName
	}
	if strings.TrimSpace(result.SiteNameZh) == "" {
		result.SiteNameZh = result.SiteName
	}
	if strings.TrimSpace(result.SiteNameEn) == "" {
		result.SiteNameEn = result.SiteName
	}
	if locale.NormalizeLanguage(result.PreferredLanguage) == "" {
		result.PreferredLanguage = defaultPreferredLang
	}
	if strings.TrimSpace(result.AISummaryPrompt) == "" {
		result.AISummaryPrompt = defaultSummarySystemPrompt
	}
	if strings.TrimSpace(result.AIRewritePrompt) == "" {
		result.AIRewritePrompt = defaultRewriteSystemPrompt
	}
	if strings.TrimSpace(result.AdminFooterText) == "" {
		result.AdminFooterText = defaultAdminFooter
	}
	if strings.TrimSpace(result.AdminFooterTextZh) == "" {
		result.AdminFooterTextZh = result.AdminFooterText
	}
	if strings.TrimSpace(result.AdminFooterTextEn) == "" {
		result.AdminFooterTextEn = defaultAdminFooterEn
	}
	if strings.TrimSpace(result.PublicFooterText) == "" {
		result.PublicFooterText = defaultPublicFooter
	}
	if strings.TrimSpace(result.PublicFooterTextZh) == "" {
		result.PublicFooterTextZh = result.PublicFooterText
	}
	if strings.TrimSpace(result.PublicFooterTextEn) == "" {
		result.PublicFooterTextEn = defaultPublicFooterEn
	}
	if strings.TrimSpace(result.GallerySubtitle) == "" {
		result.GallerySubtitle = defaultGallerySubtitle
	}
	if strings.TrimSpace(result.GallerySubtitleZh) == "" {
		result.GallerySubtitleZh = result.GallerySubtitle
	}
	if strings.TrimSpace(result.GallerySubtitleEn) == "" {
		result.GallerySubtitleEn = result.GallerySubtitle
	}

	return result, nil
}

// PreferredLanguage returns the explicitly saved preferred language, if any.
func (s *SystemSettingService) PreferredLanguage() (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, nil
	}

	var record db.SystemSetting
	if err := s.db.Select("value").Where("key = ?", db.SettingKeyPreferredLanguage).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("load preferred language: %w", err)
	}

	language := locale.NormalizeLanguage(record.Value)
	if language == "" {
		return "", false, nil
	}
	return language, true, nil
}

// UpdateSettings 保存系统设置，未填写站点名称时回退默认值。
func (s *SystemSettingService) UpdateSettings(input SystemSettingsInput) (SystemSettings, error) {
	provider := normalizeAIProvider(input.AIProvider)
	if provider == "" {
		provider = AIProviderOpenAI
	}

	galleryEnabled := defaultGalleryEnabled
	if input.GalleryEnabled != nil {
		galleryEnabled = *input.GalleryEnabled
	}
	preferredLanguage := locale.NormalizeLanguage(input.PreferredLanguage)
	if preferredLanguage == "" {
		preferredLanguage = defaultPreferredLang
	}

	siteNameZh := strings.TrimSpace(input.SiteNameZh)
	siteNameEn := strings.TrimSpace(input.SiteNameEn)
	siteName := strings.TrimSpace(input.SiteName)
	if siteName == "" {
		if siteNameZh != "" {
			siteName = siteNameZh
		} else {
			siteName = siteNameEn
		}
	}

	adminFooterZh := strings.TrimSpace(input.AdminFooterTextZh)
	adminFooterEn := strings.TrimSpace(input.AdminFooterTextEn)
	adminFooter := strings.TrimSpace(input.AdminFooterText)
	if adminFooter == "" {
		if adminFooterZh != "" {
			adminFooter = adminFooterZh
		} else {
			adminFooter = adminFooterEn
		}
	}

	publicFooterZh := strings.TrimSpace(input.PublicFooterTextZh)
	publicFooterEn := strings.TrimSpace(input.PublicFooterTextEn)
	publicFooter := strings.TrimSpace(input.PublicFooterText)
	if publicFooter == "" {
		if publicFooterZh != "" {
			publicFooter = publicFooterZh
		} else {
			publicFooter = publicFooterEn
		}
	}

	siteDescriptionZh := strings.TrimSpace(input.SiteDescriptionZh)
	siteDescriptionEn := strings.TrimSpace(input.SiteDescriptionEn)
	siteDescription := strings.TrimSpace(input.SiteDescription)
	if siteDescription == "" {
		if siteDescriptionZh != "" {
			siteDescription = siteDescriptionZh
		} else {
			siteDescription = siteDescriptionEn
		}
	}

	gallerySubtitleZh := strings.TrimSpace(input.GallerySubtitleZh)
	gallerySubtitleEn := strings.TrimSpace(input.GallerySubtitleEn)
	gallerySubtitle := strings.TrimSpace(input.GallerySubtitle)
	if gallerySubtitle == "" {
		if gallerySubtitleZh != "" {
			gallerySubtitle = gallerySubtitleZh
		} else {
			gallerySubtitle = gallerySubtitleEn
		}
	}

	sanitized := SystemSettings{
		SiteName:           siteName,
		SiteNameZh:         siteNameZh,
		SiteNameEn:         siteNameEn,
		SiteLogoURL:        strings.TrimSpace(input.SiteLogoURL),
		SiteLogoURLLight:   strings.TrimSpace(input.SiteLogoURLLight),
		SiteLogoURLDark:    strings.TrimSpace(input.SiteLogoURLDark),
		SiteDescription:    siteDescription,
		SiteDescriptionZh:  siteDescriptionZh,
		SiteDescriptionEn:  siteDescriptionEn,
		SiteSocialImage:    strings.TrimSpace(input.SiteSocialImage),
		PreferredLanguage:  preferredLanguage,
		AIProvider:         provider,
		OpenAIAPIKey:       strings.TrimSpace(input.OpenAIAPIKey),
		DeepSeekAPIKey:     strings.TrimSpace(input.DeepSeekAPIKey),
		AdminFooterText:    adminFooter,
		AdminFooterTextZh:  adminFooterZh,
		AdminFooterTextEn:  adminFooterEn,
		PublicFooterText:   publicFooter,
		PublicFooterTextZh: publicFooterZh,
		PublicFooterTextEn: publicFooterEn,
		AISummaryPrompt:    strings.TrimSpace(input.AISummaryPrompt),
		AIRewritePrompt:    strings.TrimSpace(input.AIRewritePrompt),
		GallerySubtitle:    gallerySubtitle,
		GallerySubtitleZh:  gallerySubtitleZh,
		GallerySubtitleEn:  gallerySubtitleEn,
		GalleryEnabled:     galleryEnabled,
	}

	if sanitized.SiteName == "" {
		sanitized.SiteName = defaultSiteName
	}
	if sanitized.SiteNameZh == "" {
		sanitized.SiteNameZh = sanitized.SiteName
	}
	if sanitized.SiteNameEn == "" {
		sanitized.SiteNameEn = sanitized.SiteName
	}
	if sanitized.SiteLogoURLLight == "" {
		sanitized.SiteLogoURLLight = sanitized.SiteLogoURL
	}
	if sanitized.SiteLogoURLDark == "" {
		sanitized.SiteLogoURLDark = sanitized.SiteLogoURLLight
	}
	if sanitized.SiteLogoURL == "" {
		sanitized.SiteLogoURL = sanitized.SiteLogoURLLight
	}
	if sanitized.SiteLogoURLLight == "" {
		sanitized.SiteLogoURLLight = sanitized.SiteLogoURLDark
	}
	if sanitized.SiteLogoURLDark == "" {
		sanitized.SiteLogoURLDark = sanitized.SiteLogoURLLight
	}
	if sanitized.SiteDescription == "" {
		sanitized.SiteDescription = defaultSiteDescription
	}
	if sanitized.SiteDescriptionZh == "" {
		sanitized.SiteDescriptionZh = sanitized.SiteDescription
	}
	if sanitized.SiteDescriptionEn == "" {
		sanitized.SiteDescriptionEn = sanitized.SiteDescription
	}
	if sanitized.AdminFooterText == "" {
		sanitized.AdminFooterText = defaultAdminFooter
	}
	if sanitized.AdminFooterTextZh == "" {
		sanitized.AdminFooterTextZh = sanitized.AdminFooterText
	}
	if sanitized.AdminFooterTextEn == "" {
		sanitized.AdminFooterTextEn = defaultAdminFooterEn
	}
	if sanitized.PublicFooterText == "" {
		sanitized.PublicFooterText = defaultPublicFooter
	}
	if sanitized.PublicFooterTextZh == "" {
		sanitized.PublicFooterTextZh = sanitized.PublicFooterText
	}
	if sanitized.PublicFooterTextEn == "" {
		sanitized.PublicFooterTextEn = defaultPublicFooterEn
	}
	if sanitized.GallerySubtitle == "" {
		sanitized.GallerySubtitle = defaultGallerySubtitle
	}
	if sanitized.GallerySubtitleZh == "" {
		sanitized.GallerySubtitleZh = sanitized.GallerySubtitle
	}
	if sanitized.GallerySubtitleEn == "" {
		sanitized.GallerySubtitleEn = sanitized.GallerySubtitle
	}
	if sanitized.AISummaryPrompt == "" {
		sanitized.AISummaryPrompt = defaultSummarySystemPrompt
	}
	if sanitized.AIRewritePrompt == "" {
		sanitized.AIRewritePrompt = defaultRewriteSystemPrompt
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertSetting(tx, db.SettingKeySiteName, sanitized.SiteName); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteNameZh, sanitized.SiteNameZh); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteNameEn, sanitized.SiteNameEn); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteLogoURL, sanitized.SiteLogoURL); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteLogoURLLight, sanitized.SiteLogoURLLight); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteLogoURLDark, sanitized.SiteLogoURLDark); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteDescription, sanitized.SiteDescription); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteDescriptionZh, sanitized.SiteDescriptionZh); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteDescriptionEn, sanitized.SiteDescriptionEn); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteSocialImage, sanitized.SiteSocialImage); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyPreferredLanguage, sanitized.PreferredLanguage); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteAdminFooter, sanitized.AdminFooterText); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteAdminFooterZh, sanitized.AdminFooterTextZh); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteAdminFooterEn, sanitized.AdminFooterTextEn); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySitePublicFooter, sanitized.PublicFooterText); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySitePublicFooterZh, sanitized.PublicFooterTextZh); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySitePublicFooterEn, sanitized.PublicFooterTextEn); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyGallerySubtitle, sanitized.GallerySubtitle); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyGallerySubtitleZh, sanitized.GallerySubtitleZh); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyGallerySubtitleEn, sanitized.GallerySubtitleEn); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyAIProvider, sanitized.AIProvider); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyOpenAIAPIKey, sanitized.OpenAIAPIKey); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyDeepSeekAPIKey, sanitized.DeepSeekAPIKey); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyAISummaryPrompt, sanitized.AISummaryPrompt); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyAIRewritePrompt, sanitized.AIRewritePrompt); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyGalleryEnabled, strconv.FormatBool(sanitized.GalleryEnabled)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return SystemSettings{}, fmt.Errorf("update system settings: %w", err)
	}

	return sanitized, nil
}

func upsertSetting(tx *gorm.DB, key, value string) error {
	setting := db.SystemSetting{Key: key, Value: value}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"value":      value,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(&setting).Error; err != nil {
		return fmt.Errorf("upsert setting %s: %w", key, err)
	}
	return nil
}

// SetHTTPClient 替换用于访问第三方服务的 HTTP 客户端，主要面向测试场景。
func (s *SystemSettingService) SetHTTPClient(client httpDoer) {
	if client == nil {
		s.httpClient = &http.Client{Timeout: 10 * time.Second}
		return
	}
	s.httpClient = client
}

// SetOpenAIBaseURL 覆盖 OpenAI API 的基础地址，便于测试或自定义代理。
func (s *SystemSettingService) SetOpenAIBaseURL(base string) {
	trimmed := strings.TrimSpace(base)
	s.openAIBaseURL = strings.TrimRight(trimmed, "/")
}

// SetDeepSeekBaseURL 覆盖 DeepSeek API 的基础地址，便于测试或自定义代理。
func (s *SystemSettingService) SetDeepSeekBaseURL(base string) {
	trimmed := strings.TrimSpace(base)
	s.deepSeekBaseURL = strings.TrimRight(trimmed, "/")
}

// TestAIConnection 调用指定 AI 平台的模型接口验证 API Key 的有效性。
func (s *SystemSettingService) TestAIConnection(ctx context.Context, provider, apiKey string) error {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return ErrAIAPIKeyMissing
	}

	prov := normalizeAIProvider(provider)
	if prov == "" {
		prov = AIProviderOpenAI
	}

	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	base := ""
	label := ""
	switch prov {
	case AIProviderDeepSeek:
		base = s.deepSeekBaseURL
		if strings.TrimSpace(base) == "" {
			base = "https://api.deepseek.com/v1"
		}
		label = "DeepSeek"
	default:
		base = s.openAIBaseURL
		if strings.TrimSpace(base) == "" {
			base = "https://api.openai.com/v1"
		}
		label = "OpenAI"
	}

	endpoint := strings.TrimRight(base, "/") + "/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build %s request: %w", strings.ToLower(label), err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "commitlog-admin/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 %s 接口失败: %w", label, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("%s 返回错误：%s (%s)", label, resp.Status, msg)
		}
		return fmt.Errorf("%s 返回错误：%s", label, resp.Status)
	}

	return nil
}

// TestOpenAIConnection 兼容旧方法，默认测试 OpenAI。
func (s *SystemSettingService) TestOpenAIConnection(ctx context.Context, apiKey string) error {
	return s.TestAIConnection(ctx, AIProviderOpenAI, apiKey)
}

func normalizeAIProvider(provider string) string {
	trimmed := strings.ToLower(strings.TrimSpace(provider))
	for _, candidate := range supportedAIProviders {
		if trimmed == candidate {
			return candidate
		}
	}
	return ""
}
