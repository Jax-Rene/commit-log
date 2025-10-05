package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
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

// SystemSettings 描述后台可配置的系统信息。
type SystemSettings struct {
	SiteName         string
	SiteLogoURL      string
	SiteLogoURLLight string
	SiteLogoURLDark  string
	AIProvider       string
	OpenAIAPIKey     string
	DeepSeekAPIKey   string
	AdminFooterText  string
	PublicFooterText string
}

// ErrAIAPIKeyMissing 表示未提供必需的 AI 平台 API Key。
var ErrAIAPIKeyMissing = errors.New("api key is required")

// ErrOpenAIAPIKeyMissing 为历史兼容，等价于 ErrAIAPIKeyMissing。
var ErrOpenAIAPIKeyMissing = ErrAIAPIKeyMissing

// SystemSettingsInput 用于更新系统设置。
type SystemSettingsInput struct {
	SiteName         string
	SiteLogoURL      string
	SiteLogoURLLight string
	SiteLogoURLDark  string
	AIProvider       string
	OpenAIAPIKey     string
	DeepSeekAPIKey   string
	AdminFooterText  string
	PublicFooterText string
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
	db.SettingKeySiteLogoURL,
	db.SettingKeySiteLogoURLLight,
	db.SettingKeySiteLogoURLDark,
	db.SettingKeySiteAdminFooter,
	db.SettingKeySitePublicFooter,
	db.SettingKeyAIProvider,
	db.SettingKeyOpenAIAPIKey,
	db.SettingKeyDeepSeekAPIKey,
}

// GetSettings 读取系统设置，如未设置将返回默认值。
func (s *SystemSettingService) GetSettings() (SystemSettings, error) {
	result := SystemSettings{
		SiteName:         "CommitLog",
		AIProvider:       AIProviderOpenAI,
		AdminFooterText:  "日拱一卒，功不唐捐",
		PublicFooterText: "激发创造，延迟满足",
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
		case db.SettingKeySiteLogoURL:
			result.SiteLogoURL = record.Value
		case db.SettingKeySiteLogoURLLight:
			result.SiteLogoURLLight = record.Value
		case db.SettingKeySiteLogoURLDark:
			result.SiteLogoURLDark = record.Value
		case db.SettingKeySiteAdminFooter:
			if strings.TrimSpace(record.Value) != "" {
				result.AdminFooterText = record.Value
			}
		case db.SettingKeySitePublicFooter:
			if strings.TrimSpace(record.Value) != "" {
				result.PublicFooterText = record.Value
			}
		case db.SettingKeyAIProvider:
			if provider := normalizeAIProvider(record.Value); provider != "" {
				result.AIProvider = provider
			}
		case db.SettingKeyOpenAIAPIKey:
			result.OpenAIAPIKey = record.Value
		case db.SettingKeyDeepSeekAPIKey:
			result.DeepSeekAPIKey = record.Value
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

	return result, nil
}

// UpdateSettings 保存系统设置，未填写站点名称时回退默认值。
func (s *SystemSettingService) UpdateSettings(input SystemSettingsInput) (SystemSettings, error) {
	provider := normalizeAIProvider(input.AIProvider)
	if provider == "" {
		provider = AIProviderOpenAI
	}

	sanitized := SystemSettings{
		SiteName:         strings.TrimSpace(input.SiteName),
		SiteLogoURL:      strings.TrimSpace(input.SiteLogoURL),
		SiteLogoURLLight: strings.TrimSpace(input.SiteLogoURLLight),
		SiteLogoURLDark:  strings.TrimSpace(input.SiteLogoURLDark),
		AIProvider:       provider,
		OpenAIAPIKey:     strings.TrimSpace(input.OpenAIAPIKey),
		DeepSeekAPIKey:   strings.TrimSpace(input.DeepSeekAPIKey),
		AdminFooterText:  strings.TrimSpace(input.AdminFooterText),
		PublicFooterText: strings.TrimSpace(input.PublicFooterText),
	}

	if sanitized.SiteName == "" {
		sanitized.SiteName = "CommitLog"
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
	if sanitized.AdminFooterText == "" {
		sanitized.AdminFooterText = "日拱一卒，功不唐捐"
	}
	if sanitized.PublicFooterText == "" {
		sanitized.PublicFooterText = "激发创造，延迟满足"
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertSetting(tx, db.SettingKeySiteName, sanitized.SiteName); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteLogoURL, sanitized.SiteLogoURLLight); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteLogoURLLight, sanitized.SiteLogoURLLight); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteLogoURLDark, sanitized.SiteLogoURLDark); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteAdminFooter, sanitized.AdminFooterText); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySitePublicFooter, sanitized.PublicFooterText); err != nil {
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
