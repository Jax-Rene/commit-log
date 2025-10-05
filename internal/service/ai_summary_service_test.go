package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/commitlog/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeHTTPClient struct {
	handler func(*http.Request) (*http.Response, error)
}

func (f fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if f.handler == nil {
		return nil, errors.New("no handler configured")
	}
	return f.handler(req)
}

func setupAISummaryTestDB(t *testing.T) func() {
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

func TestAISummaryServiceGenerateSummary(t *testing.T) {
	cleanup := setupAISummaryTestDB(t)
	defer cleanup()

	system := NewSystemSettingService(db.DB)
	if _, err := system.UpdateSettings(SystemSettingsInput{SiteName: "CommitLog", SiteLogoURL: "https://example.com/logo.png", OpenAIAPIKey: "sk-test"}); err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	svc := NewAISummaryService(system)
	svc.SetBaseURL("https://openai.test/v1")
	svc.SetHTTPClient(fakeHTTPClient{handler: func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("unexpected authorization header %s", got)
		}

		var payload chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Model == "" {
			t.Fatalf("expected model to be set")
		}

		response := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage "json:\"message\""
			}{{Message: chatMessage{Role: "assistant", Content: "这是一段自动生成的摘要。"}}},
			Usage: struct {
				PromptTokens     int "json:\"prompt_tokens\""
				CompletionTokens int "json:\"completion_tokens\""
			}{PromptTokens: 100, CompletionTokens: 30},
		}
		buf, _ := json.Marshal(response)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(buf)),
			Header:     make(http.Header),
		}, nil
	}})

	result, err := svc.GenerateSummary(context.Background(), SummaryInput{Title: "测试标题", Content: "这里是具体内容"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary != "这是一段自动生成的摘要。" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.PromptTokens != 100 || result.CompletionTokens != 30 {
		t.Fatalf("unexpected usage: %+v", result)
	}
}

func TestAISummaryServiceMissingAPIKey(t *testing.T) {
	cleanup := setupAISummaryTestDB(t)
	defer cleanup()

	system := NewSystemSettingService(db.DB)
	svc := NewAISummaryService(system)

	_, err := svc.GenerateSummary(context.Background(), SummaryInput{Title: "测试", Content: "内容"})
	if err == nil {
		t.Fatal("expected error when api key missing")
	}
	if err != ErrOpenAIAPIKeyMissing {
		t.Fatalf("unexpected error: %v", err)
	}
}
