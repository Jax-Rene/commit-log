package service

import (
	"bytes"
	"context"
	"encoding/json"
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

func setupAIRewriteTestDB(t *testing.T) func() {
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

func TestAIRewriteServiceOptimizeContent(t *testing.T) {
	cleanup := setupAIRewriteTestDB(t)
	defer cleanup()

	system := NewSystemSettingService(db.DB)
	if _, err := system.UpdateSettings(SystemSettingsInput{AIProvider: AIProviderOpenAI, OpenAIAPIKey: "sk-test"}); err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	svc := NewAIRewriteService(system)
	svc.SetOpenAIBaseURL("https://openai.test/v1")
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
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(payload.Messages))
		}
		if payload.Messages[0].Role != "system" {
			t.Fatalf("unexpected system role: %s", payload.Messages[0].Role)
		}
		if payload.Messages[1].Role != "user" {
			t.Fatalf("unexpected user role: %s", payload.Messages[1].Role)
		}
		if payload.MaxTokens != defaultOptimizationMaxTokens {
			t.Fatalf("unexpected max tokens: %d", payload.MaxTokens)
		}
		if strings.Contains(payload.Messages[1].Content, "测试标题") {
			t.Fatalf("user prompt should not contain title: %q", payload.Messages[1].Content)
		}
		if !strings.Contains(payload.Messages[1].Content, "原始内容") {
			t.Fatalf("user prompt must include content body: %q", payload.Messages[1].Content)
		}

		response := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage "json:\"message\""
			}{{Message: chatMessage{Role: "assistant", Content: "优化后的内容"}}},
			Usage: struct {
				PromptTokens     int "json:\"prompt_tokens\""
				CompletionTokens int "json:\"completion_tokens\""
			}{PromptTokens: 512, CompletionTokens: 1024},
		}
		buf, _ := json.Marshal(response)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(buf)),
			Header:     make(http.Header),
		}, nil
	}})

	result, err := svc.OptimizeContent(context.Background(), ContentOptimizationInput{
		Title:   "测试标题",
		Content: "原始内容",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "优化后的内容" {
		t.Fatalf("unexpected content: %s", result.Content)
	}
	if result.PromptTokens != 512 || result.CompletionTokens != 1024 {
		t.Fatalf("unexpected usage: %+v", result)
	}
}

func TestAIRewriteServiceMissingAPIKey(t *testing.T) {
	cleanup := setupAIRewriteTestDB(t)
	defer cleanup()

	system := NewSystemSettingService(db.DB)
	svc := NewAIRewriteService(system)

	_, err := svc.OptimizeContent(context.Background(), ContentOptimizationInput{
		Title:   "测试标题",
		Content: "一些正文",
	})
	if err == nil {
		t.Fatal("expected error when api key missing")
	}
	if !errors.Is(err, ErrAIAPIKeyMissing) {
		t.Fatalf("unexpected error: %v", err)
	}
}
