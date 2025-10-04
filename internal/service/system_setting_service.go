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

// SystemSettings 描述后台可配置的系统信息。
type SystemSettings struct {
	SiteName     string
	SiteLogoURL  string
	OpenAIAPIKey string
}

// ErrOpenAIAPIKeyMissing 表示未提供必需的 OpenAI API Key。
var ErrOpenAIAPIKeyMissing = errors.New("api key is required")

// SystemSettingsInput 用于更新系统设置。
type SystemSettingsInput struct {
	SiteName     string
	SiteLogoURL  string
	OpenAIAPIKey string
}

// SystemSettingService 提供系统设置的读取与更新能力。
type SystemSettingService struct {
	db            *gorm.DB
	httpClient    httpDoer
	openAIBaseURL string
}

// NewSystemSettingService 构造 SystemSettingService。
func NewSystemSettingService(gdb *gorm.DB) *SystemSettingService {
	return &SystemSettingService{
		db:            gdb,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		openAIBaseURL: "https://api.openai.com/v1",
	}
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

var settingKeys = []string{
	db.SettingKeySiteName,
	db.SettingKeySiteLogoURL,
	db.SettingKeyOpenAIAPIKey,
}

// GetSettings 读取系统设置，如未设置将返回默认值。
func (s *SystemSettingService) GetSettings() (SystemSettings, error) {
	result := SystemSettings{SiteName: "CommitLog"}

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
		case db.SettingKeyOpenAIAPIKey:
			result.OpenAIAPIKey = record.Value
		}
	}

	return result, nil
}

// UpdateSettings 保存系统设置，未填写站点名称时回退默认值。
func (s *SystemSettingService) UpdateSettings(input SystemSettingsInput) (SystemSettings, error) {
	sanitized := SystemSettings{
		SiteName:     strings.TrimSpace(input.SiteName),
		SiteLogoURL:  strings.TrimSpace(input.SiteLogoURL),
		OpenAIAPIKey: strings.TrimSpace(input.OpenAIAPIKey),
	}

	if sanitized.SiteName == "" {
		sanitized.SiteName = "CommitLog"
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertSetting(tx, db.SettingKeySiteName, sanitized.SiteName); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeySiteLogoURL, sanitized.SiteLogoURL); err != nil {
			return err
		}
		if err := upsertSetting(tx, db.SettingKeyOpenAIAPIKey, sanitized.OpenAIAPIKey); err != nil {
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

// TestOpenAIConnection 调用 OpenAI 模型列表接口验证 API Key 的有效性与可用性。
func (s *SystemSettingService) TestOpenAIConnection(ctx context.Context, apiKey string) error {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return ErrOpenAIAPIKeyMissing
	}

	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	base := s.openAIBaseURL
	if strings.TrimSpace(base) == "" {
		base = "https://api.openai.com/v1"
	}
	endpoint := strings.TrimRight(base, "/") + "/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "commitlog-admin/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 OpenAI 接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("OpenAI 返回错误：%s (%s)", resp.Status, msg)
		}
		return fmt.Errorf("OpenAI 返回错误：%s", resp.Status)
	}

	return nil
}
