package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/locale"
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
	if settings.SiteNameZh != "CommitLog" || settings.SiteNameEn != "CommitLog" {
		t.Fatalf("expected localized site name defaults to CommitLog, got zh=%q en=%q", settings.SiteNameZh, settings.SiteNameEn)
	}
	if settings.SiteDescription != defaultSiteDescription {
		t.Fatalf("expected default site description %q, got %q", defaultSiteDescription, settings.SiteDescription)
	}
	if settings.SiteDescriptionZh != defaultSiteDescription || settings.SiteDescriptionEn != defaultSiteDescription {
		t.Fatalf("expected localized site description defaults to %q, got zh=%q en=%q", defaultSiteDescription, settings.SiteDescriptionZh, settings.SiteDescriptionEn)
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
	if settings.AdminFooterTextZh != "日拱一卒，功不唐捐" || settings.AdminFooterTextEn == "" {
		t.Fatalf("unexpected localized admin footer defaults: zh=%q en=%q", settings.AdminFooterTextZh, settings.AdminFooterTextEn)
	}
	if settings.PublicFooterText != "激发创造，延迟满足" {
		t.Fatalf("unexpected public footer default: %q", settings.PublicFooterText)
	}
	if settings.PublicFooterTextZh != "激发创造，延迟满足" || settings.PublicFooterTextEn == "" {
		t.Fatalf("unexpected localized public footer defaults: zh=%q en=%q", settings.PublicFooterTextZh, settings.PublicFooterTextEn)
	}
	if settings.AIProvider != AIProviderOpenAI {
		t.Fatalf("expected default provider openai, got %s", settings.AIProvider)
	}
	if settings.AISummaryPrompt != defaultSummarySystemPrompt {
		t.Fatalf("unexpected summary prompt default: %q", settings.AISummaryPrompt)
	}
	if settings.AIRewritePrompt != defaultRewriteSystemPrompt {
		t.Fatalf("unexpected rewrite prompt default: %q", settings.AIRewritePrompt)
	}
	if settings.GallerySubtitle != defaultGallerySubtitle {
		t.Fatalf("unexpected gallery subtitle default: %q", settings.GallerySubtitle)
	}
	if settings.GallerySubtitleZh != defaultGallerySubtitle || settings.GallerySubtitleEn != defaultGallerySubtitle {
		t.Fatalf("unexpected localized gallery subtitle defaults: zh=%q en=%q", settings.GallerySubtitleZh, settings.GallerySubtitleEn)
	}
	if !settings.GalleryEnabled {
		t.Fatalf("expected gallery to be enabled by default")
	}
	if settings.PreferredLanguage != locale.LanguageChinese {
		t.Fatalf("expected preferred language %q, got %q", locale.LanguageChinese, settings.PreferredLanguage)
	}
}

func TestSystemSettingServicePreferredLanguageExplicit(t *testing.T) {
	cleanup := setupSystemSettingTestDB(t)
	defer cleanup()

	svc := NewSystemSettingService(db.DB)
	language, ok, err := svc.PreferredLanguage()
	if err != nil {
		t.Fatalf("preferred language failed: %v", err)
	}
	if ok || language != "" {
		t.Fatalf("expected no preferred language, got %q (ok=%v)", language, ok)
	}

	if _, err := svc.UpdateSettings(SystemSettingsInput{PreferredLanguage: "en"}); err != nil {
		t.Fatalf("update preferred language failed: %v", err)
	}

	language, ok, err = svc.PreferredLanguage()
	if err != nil {
		t.Fatalf("preferred language failed: %v", err)
	}
	if !ok || language != locale.LanguageEnglish {
		t.Fatalf("expected preferred language %q (ok=true), got %q (ok=%v)", locale.LanguageEnglish, language, ok)
	}
}

func TestSystemSettingServiceGetSettingsWithoutDB(t *testing.T) {
	svc := NewSystemSettingService(nil)
	settings, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("get settings failed: %v", err)
	}
	if settings.SiteName != defaultSiteName {
		t.Fatalf("expected default site name %q, got %q", defaultSiteName, settings.SiteName)
	}
	if settings.PreferredLanguage != locale.LanguageChinese {
		t.Fatalf("expected preferred language %q, got %q", locale.LanguageChinese, settings.PreferredLanguage)
	}
}

func TestSystemSettingServiceUpdateAndRetrieve(t *testing.T) {
	cleanup := setupSystemSettingTestDB(t)
	defer cleanup()

	svc := NewSystemSettingService(db.DB)
	input := SystemSettingsInput{
		SiteName:           " CommitLog 社区 ",
		SiteNameZh:         " 提交日志 ",
		SiteNameEn:         " CommitLog Community ",
		SiteLogoURL:        "https://example.com/logo.png",
		SiteLogoURLLight:   "https://example.com/logo-light.png",
		SiteLogoURLDark:    "https://example.com/logo-dark.png",
		SiteDescription:    " 致力于分享 AI 工程实战 ",
		SiteDescriptionZh:  " 中文描述 ",
		SiteDescriptionEn:  " Description EN ",
		SiteSocialImage:    "https://example.com/og.png",
		AdminFooterText:    "后台页脚",
		AdminFooterTextZh:  " 后台页脚中文 ",
		AdminFooterTextEn:  " Admin footer ",
		PublicFooterText:   "前台页脚",
		PublicFooterTextZh: " 前台页脚中文 ",
		PublicFooterTextEn: " Public footer ",
		GallerySubtitle:    " Shot by Hasselblad X2D / iPhone 16 ",
		GallerySubtitleZh:  " 中文副标题 ",
		GallerySubtitleEn:  " Subtitle EN ",
		PreferredLanguage:  "en",
		AIProvider:         "deepseek",
		OpenAIAPIKey:       "sk-xxxx",
		DeepSeekAPIKey:     "ds-12345",
		GalleryEnabled:     boolPtr(false),
		AISummaryPrompt:    " 摘要提示 ",
		AIRewritePrompt:    " 重写提示 ",
	}

	saved, err := svc.UpdateSettings(input)
	if err != nil {
		t.Fatalf("update settings failed: %v", err)
	}

	if saved.SiteName != "CommitLog 社区" {
		t.Fatalf("expected sanitized site name, got %q", saved.SiteName)
	}
	if saved.SiteNameZh != "提交日志" || saved.SiteNameEn != "CommitLog Community" {
		t.Fatalf("expected localized site names to be sanitized, got zh=%q en=%q", saved.SiteNameZh, saved.SiteNameEn)
	}
	if saved.SiteDescription != "致力于分享 AI 工程实战" {
		t.Fatalf("expected sanitized description, got %q", saved.SiteDescription)
	}
	if saved.SiteDescriptionZh != "中文描述" || saved.SiteDescriptionEn != "Description EN" {
		t.Fatalf("expected localized description sanitized, got zh=%q en=%q", saved.SiteDescriptionZh, saved.SiteDescriptionEn)
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
	if saved.GalleryEnabled {
		t.Fatalf("expected gallery to be disabled, got %v", saved.GalleryEnabled)
	}
	if saved.AISummaryPrompt != "摘要提示" {
		t.Fatalf("expected summary prompt sanitized, got %q", saved.AISummaryPrompt)
	}
	if saved.AIRewritePrompt != "重写提示" {
		t.Fatalf("expected rewrite prompt sanitized, got %q", saved.AIRewritePrompt)
	}
	if saved.GallerySubtitle != "Shot by Hasselblad X2D / iPhone 16" {
		t.Fatalf("expected gallery subtitle sanitized, got %q", saved.GallerySubtitle)
	}
	if saved.GallerySubtitleZh != "中文副标题" || saved.GallerySubtitleEn != "Subtitle EN" {
		t.Fatalf("expected localized gallery subtitle sanitized, got zh=%q en=%q", saved.GallerySubtitleZh, saved.GallerySubtitleEn)
	}
	if saved.PreferredLanguage != locale.LanguageEnglish {
		t.Fatalf("expected preferred language %q, got %q", locale.LanguageEnglish, saved.PreferredLanguage)
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
	if fetched.SiteDescriptionZh != "中文描述" || fetched.SiteDescriptionEn != "Description EN" {
		t.Fatalf("expected localized description, got zh=%q en=%q", fetched.SiteDescriptionZh, fetched.SiteDescriptionEn)
	}
	if fetched.SiteSocialImage != "https://example.com/og.png" {
		t.Fatalf("expected social image %q, got %q", "https://example.com/og.png", fetched.SiteSocialImage)
	}
	if fetched.AdminFooterText != input.AdminFooterText {
		t.Fatalf("expected admin footer %q, got %q", input.AdminFooterText, fetched.AdminFooterText)
	}
	if fetched.AdminFooterTextZh != "后台页脚中文" || fetched.AdminFooterTextEn != "Admin footer" {
		t.Fatalf("expected localized admin footer, got zh=%q en=%q", fetched.AdminFooterTextZh, fetched.AdminFooterTextEn)
	}
	if fetched.PublicFooterText != input.PublicFooterText {
		t.Fatalf("expected public footer %q, got %q", input.PublicFooterText, fetched.PublicFooterText)
	}
	if fetched.PublicFooterTextZh != "前台页脚中文" || fetched.PublicFooterTextEn != "Public footer" {
		t.Fatalf("expected localized public footer, got zh=%q en=%q", fetched.PublicFooterTextZh, fetched.PublicFooterTextEn)
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
	if fetched.GalleryEnabled {
		t.Fatalf("expected gallery to remain disabled, got %v", fetched.GalleryEnabled)
	}
	if fetched.AISummaryPrompt != "摘要提示" {
		t.Fatalf("expected summary prompt %q, got %q", "摘要提示", fetched.AISummaryPrompt)
	}
	if fetched.AIRewritePrompt != "重写提示" {
		t.Fatalf("expected rewrite prompt %q, got %q", "重写提示", fetched.AIRewritePrompt)
	}
	if fetched.GallerySubtitle != "Shot by Hasselblad X2D / iPhone 16" {
		t.Fatalf("expected gallery subtitle %q, got %q", "Shot by Hasselblad X2D / iPhone 16", fetched.GallerySubtitle)
	}
	if fetched.GallerySubtitleZh != "中文副标题" || fetched.GallerySubtitleEn != "Subtitle EN" {
		t.Fatalf("expected localized gallery subtitle, got zh=%q en=%q", fetched.GallerySubtitleZh, fetched.GallerySubtitleEn)
	}
	if fetched.PreferredLanguage != locale.LanguageEnglish {
		t.Fatalf("expected preferred language %q, got %q", locale.LanguageEnglish, fetched.PreferredLanguage)
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
	if saved.SiteSocialImage != "" {
		t.Fatalf("expected social image fallback to empty string, got %q", saved.SiteSocialImage)
	}
	if saved.AIProvider != AIProviderOpenAI {
		t.Fatalf("expected provider fallback to openai, got %q", saved.AIProvider)
	}
	if !saved.GalleryEnabled {
		t.Fatalf("expected gallery to stay enabled by default, got %v", saved.GalleryEnabled)
	}
	if saved.AISummaryPrompt != defaultSummarySystemPrompt {
		t.Fatalf("expected summary prompt fallback to default, got %q", saved.AISummaryPrompt)
	}
	if saved.AIRewritePrompt != defaultRewriteSystemPrompt {
		t.Fatalf("expected rewrite prompt fallback to default, got %q", saved.AIRewritePrompt)
	}
	if saved.GallerySubtitle != defaultGallerySubtitle {
		t.Fatalf("expected gallery subtitle fallback to default, got %q", saved.GallerySubtitle)
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
