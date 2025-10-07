package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupSystemSettingTestDB(t *testing.T) func() {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.SystemSetting{}); err != nil {
		t.Fatalf("failed to migrate system settings: %v", err)
	}

	db.DB = gdb

	return func() {
		sqlDB, err := db.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestSystemSettingServiceDefaults(t *testing.T) {
	cleanup := setupSystemSettingTestDB(t)
	defer cleanup()

	svc := NewSystemSettingService(db.DB)
	settings, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("get settings failed: %v", err)
	}

	if settings.SiteName != "CommitLog" {
		t.Fatalf("expected default site name CommitLog, got %s", settings.SiteName)
	}
	if settings.SiteDescription != defaultSiteDescription {
		t.Fatalf("expected default site description %q, got %q", defaultSiteDescription, settings.SiteDescription)
	}
	if settings.SiteKeywords != NormalizeKeywords(defaultSiteKeywords) {
		t.Fatalf("expected default keywords %q, got %q", NormalizeKeywords(defaultSiteKeywords), settings.SiteKeywords)
	}
	if settings.SiteLogoURL != "" || settings.SiteLogoURLLight != "" || settings.SiteLogoURLDark != "" || settings.OpenAIAPIKey != "" || settings.DeepSeekAPIKey != "" {
		t.Fatalf("expected keys to be empty, got %#v", settings)
	}
	if settings.SiteSocialImage != "" {
		t.Fatalf("expected default social image to be empty")
	}
	if settings.AdminFooterText != "日拱一卒，功不唐捐" {
		t.Fatalf("unexpected admin footer default: %q", settings.AdminFooterText)
	}
	if settings.PublicFooterText != "激发创造，延迟满足" {
		t.Fatalf("unexpected public footer default: %q", settings.PublicFooterText)
	}
	if settings.AIProvider != AIProviderOpenAI {
		t.Fatalf("expected default provider openai, got %s", settings.AIProvider)
	}
}

func TestSystemSettingServiceUpdateAndRetrieve(t *testing.T) {
	cleanup := setupSystemSettingTestDB(t)
	defer cleanup()

	svc := NewSystemSettingService(db.DB)
	input := SystemSettingsInput{
		SiteName:         " CommitLog 社区 ",
		SiteLogoURL:      "https://example.com/logo.png",
		SiteLogoURLLight: "https://example.com/logo-light.png",
		SiteLogoURLDark:  "https://example.com/logo-dark.png",
		SiteDescription:  " 致力于分享 AI 工程实战 ",
		SiteKeywords:     "AI, 工程, 博客, AI",
		SiteSocialImage:  "https://example.com/og.png",
		AdminFooterText:  "后台页脚",
		PublicFooterText: "前台页脚",
		AIProvider:       "deepseek",
		OpenAIAPIKey:     "sk-xxxx",
		DeepSeekAPIKey:   "ds-12345",
	}

	saved, err := svc.UpdateSettings(input)
	if err != nil {
		t.Fatalf("update settings failed: %v", err)
	}

	if saved.SiteName != "CommitLog 社区" {
		t.Fatalf("expected sanitized site name, got %q", saved.SiteName)
	}
	if saved.SiteDescription != "致力于分享 AI 工程实战" {
		t.Fatalf("expected sanitized description, got %q", saved.SiteDescription)
	}
	if saved.SiteKeywords != "AI, 工程, 博客" {
		t.Fatalf("expected normalized keywords, got %q", saved.SiteKeywords)
	}
	if saved.SiteSocialImage != "https://example.com/og.png" {
		t.Fatalf("expected social image %q, got %q", input.SiteSocialImage, saved.SiteSocialImage)
	}
	if saved.AIProvider != AIProviderDeepSeek {
		t.Fatalf("expected provider to be deepseek, got %q", saved.AIProvider)
	}
	if saved.DeepSeekAPIKey != "ds-12345" {
		t.Fatalf("expected deepseek key to be persisted, got %q", saved.DeepSeekAPIKey)
	}

	fetched, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("get settings failed: %v", err)
	}

	if fetched.SiteLogoURL != strings.TrimSpace(input.SiteLogoURL) {
		t.Fatalf("expected legacy logo %q, got %q", strings.TrimSpace(input.SiteLogoURL), fetched.SiteLogoURL)
	}
	if fetched.SiteLogoURLLight != strings.TrimSpace(input.SiteLogoURLLight) {
		t.Fatalf("expected light logo %q, got %q", input.SiteLogoURLLight, fetched.SiteLogoURLLight)
	}
	if fetched.SiteLogoURLDark != strings.TrimSpace(input.SiteLogoURLDark) {
		t.Fatalf("expected dark logo %q, got %q", input.SiteLogoURLDark, fetched.SiteLogoURLDark)
	}
	if fetched.SiteDescription != "致力于分享 AI 工程实战" {
		t.Fatalf("expected description %q, got %q", "致力于分享 AI 工程实战", fetched.SiteDescription)
	}
	if fetched.SiteKeywords != "AI, 工程, 博客" {
		t.Fatalf("expected keywords %q, got %q", "AI, 工程, 博客", fetched.SiteKeywords)
	}
	if fetched.SiteSocialImage != "https://example.com/og.png" {
		t.Fatalf("expected social image %q, got %q", "https://example.com/og.png", fetched.SiteSocialImage)
	}
	if fetched.AdminFooterText != input.AdminFooterText {
		t.Fatalf("expected admin footer %q, got %q", input.AdminFooterText, fetched.AdminFooterText)
	}
	if fetched.PublicFooterText != input.PublicFooterText {
		t.Fatalf("expected public footer %q, got %q", input.PublicFooterText, fetched.PublicFooterText)
	}
	if fetched.AIProvider != AIProviderDeepSeek {
		t.Fatalf("expected provider %q, got %q", AIProviderDeepSeek, fetched.AIProvider)
	}
	if fetched.OpenAIAPIKey != strings.TrimSpace(input.OpenAIAPIKey) {
		t.Fatalf("expected openai api key %q, got %q", strings.TrimSpace(input.OpenAIAPIKey), fetched.OpenAIAPIKey)
	}
	if fetched.DeepSeekAPIKey != input.DeepSeekAPIKey {
		t.Fatalf("expected deepseek api key %q, got %q", input.DeepSeekAPIKey, fetched.DeepSeekAPIKey)
	}
}

func TestSystemSettingServiceFallbackSiteName(t *testing.T) {
	cleanup := setupSystemSettingTestDB(t)
	defer cleanup()

	svc := NewSystemSettingService(db.DB)
	saved, err := svc.UpdateSettings(SystemSettingsInput{SiteName: "   "})
	if err != nil {
		t.Fatalf("update settings failed: %v", err)
	}

	if saved.SiteName != "CommitLog" {
		t.Fatalf("expected site name fallback to CommitLog, got %q", saved.SiteName)
	}
	if saved.SiteDescription != defaultSiteDescription {
		t.Fatalf("expected description fallback to %q, got %q", defaultSiteDescription, saved.SiteDescription)
	}
	if saved.SiteKeywords != NormalizeKeywords(defaultSiteKeywords) {
		t.Fatalf("expected keywords fallback to %q, got %q", NormalizeKeywords(defaultSiteKeywords), saved.SiteKeywords)
	}
	if saved.SiteSocialImage != "" {
		t.Fatalf("expected social image fallback to empty string, got %q", saved.SiteSocialImage)
	}
	if saved.AIProvider != AIProviderOpenAI {
		t.Fatalf("expected provider fallback to openai, got %q", saved.AIProvider)
	}
}

type stubHTTPClient struct {
	t            *testing.T
	allowedKey   string
	expectedHost string
}

func (s stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	s.t.Helper()
	if !strings.HasSuffix(req.URL.Path, "/models") {
		s.t.Fatalf("unexpected path %s", req.URL.Path)
	}
	if s.expectedHost != "" && req.URL.Host != s.expectedHost {
		s.t.Fatalf("unexpected host %s", req.URL.Host)
	}
	auth := req.Header.Get("Authorization")
	expected := "Bearer " + s.allowedKey
	if s.allowedKey != "" && auth != expected {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader("unauthorized")),
			Header:     make(http.Header),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     make(http.Header),
	}, nil
}

func TestSystemSettingServiceTestAIConnection(t *testing.T) {
	cleanup := setupSystemSettingTestDB(t)
	defer cleanup()

	svc := NewSystemSettingService(db.DB)
	svc.SetHTTPClient(stubHTTPClient{t: t, allowedKey: "sk-valid", expectedHost: "openai.test"})
	svc.SetOpenAIBaseURL("https://openai.test/v1")

	if err := svc.TestAIConnection(context.Background(), AIProviderOpenAI, ""); !errors.Is(err, ErrAIAPIKeyMissing) {
		t.Fatalf("expected ErrAIAPIKeyMissing, got %v", err)
	}

	if err := svc.TestAIConnection(context.Background(), AIProviderOpenAI, "sk-invalid"); err == nil {
		t.Fatal("expected error for invalid key")
	}

	if err := svc.TestAIConnection(context.Background(), AIProviderOpenAI, "sk-valid"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc.SetDeepSeekBaseURL("https://deepseek.test/v1")
	svc.SetHTTPClient(stubHTTPClient{t: t, allowedKey: "ds-valid", expectedHost: "deepseek.test"})

	if err := svc.TestAIConnection(context.Background(), AIProviderDeepSeek, "ds-valid"); err != nil {
		t.Fatalf("unexpected error for deepseek: %v", err)
	}
}
