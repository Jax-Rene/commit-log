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
	if settings.SiteLogoURL != "" || settings.OpenAIAPIKey != "" {
		t.Fatalf("expected other fields to be empty, got %#v", settings)
	}
}

func TestSystemSettingServiceUpdateAndRetrieve(t *testing.T) {
	cleanup := setupSystemSettingTestDB(t)
	defer cleanup()

	svc := NewSystemSettingService(db.DB)
	input := SystemSettingsInput{
		SiteName:     " CommitLog 社区 ",
		SiteLogoURL:  "https://example.com/logo.png",
		OpenAIAPIKey: "sk-xxxx",
	}

	saved, err := svc.UpdateSettings(input)
	if err != nil {
		t.Fatalf("update settings failed: %v", err)
	}

	if saved.SiteName != "CommitLog 社区" {
		t.Fatalf("expected sanitized site name, got %q", saved.SiteName)
	}
	if saved.OpenAIAPIKey != "sk-xxxx" {
		t.Fatalf("expected api key to be persisted, got %q", saved.OpenAIAPIKey)
	}

	fetched, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("get settings failed: %v", err)
	}

	if fetched.SiteLogoURL != input.SiteLogoURL {
		t.Fatalf("expected site logo url %q, got %q", input.SiteLogoURL, fetched.SiteLogoURL)
	}
	if fetched.OpenAIAPIKey != input.OpenAIAPIKey {
		t.Fatalf("expected api key %q, got %q", input.OpenAIAPIKey, fetched.OpenAIAPIKey)
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
}

type stubHTTPClient struct {
	t *testing.T
}

func (s stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	s.t.Helper()
	if !strings.HasSuffix(req.URL.Path, "/models") {
		s.t.Fatalf("unexpected path %s", req.URL.Path)
	}
	auth := req.Header.Get("Authorization")
	if auth != "Bearer sk-valid" {
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

func TestSystemSettingServiceTestOpenAIConnection(t *testing.T) {
	cleanup := setupSystemSettingTestDB(t)
	defer cleanup()

	svc := NewSystemSettingService(db.DB)
	svc.SetHTTPClient(stubHTTPClient{t: t})
	svc.SetOpenAIBaseURL("https://openai.test/v1")

	if err := svc.TestOpenAIConnection(context.Background(), ""); !errors.Is(err, ErrOpenAIAPIKeyMissing) {
		t.Fatalf("expected ErrOpenAIAPIKeyMissing, got %v", err)
	}

	if err := svc.TestOpenAIConnection(context.Background(), "sk-invalid"); err == nil {
		t.Fatal("expected error for invalid key")
	}

	if err := svc.TestOpenAIConnection(context.Background(), "sk-valid"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
