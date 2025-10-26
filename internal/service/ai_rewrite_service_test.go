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
	if _, err := system.UpdateSettings(SystemSettingsInput{
		AIProvider:      AIProviderOpenAI,
		OpenAIAPIKey:    "sk-test",
		AIRewritePrompt: " 自定义优化提示 ",
	}); err != nil {
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
		if payload.Messages[0].Content != "自定义优化提示" {
			t.Fatalf("unexpected system prompt: %q", payload.Messages[0].Content)
		}
		if !strings.Contains(payload.Messages[1].Content, "原始内容") {
			t.Fatalf("user prompt must include content body: %q", payload.Messages[1].Content)
		}

		response := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage "json:\"message\""
			}{{Message: chatMessage{Role: "assistant", Content: "```markdown\n优化后的内容\n```"}}},
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
		Content: "一些正文",
	})
	if err == nil {
		t.Fatal("expected error when api key missing")
	}
	if !errors.Is(err, ErrAIAPIKeyMissing) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAIRewriteServiceRewriteSnippet(t *testing.T) {
	cleanup := setupAIRewriteTestDB(t)
	defer cleanup()

	system := NewSystemSettingService(db.DB)
	if _, err := system.UpdateSettings(SystemSettingsInput{
		AIProvider:      AIProviderOpenAI,
		OpenAIAPIKey:    "sk-inline",
		AIRewritePrompt: " 片段改写系统提示 ",
	}); err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	svc := NewAIRewriteService(system)
	svc.SetOpenAIBaseURL("https://openai.example/v1")

	svc.SetHTTPClient(fakeHTTPClient{handler: func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var payload chatCompletionRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if payload.MaxTokens != defaultSnippetRewriteMaxTokens {
			t.Fatalf("unexpected max tokens: %d", payload.MaxTokens)
		}
		if payload.Messages[0].Content != "片段改写系统提示" {
			t.Fatalf("unexpected system prompt: %q", payload.Messages[0].Content)
		}
		userPrompt := payload.Messages[1].Content
		if !strings.Contains(userPrompt, "原始片段内容") {
			t.Fatalf("user prompt should include selection: %q", userPrompt)
		}
		if !strings.Contains(userPrompt, "请将语气改为正式书面语") {
			t.Fatalf("user prompt should include instruction: %q", userPrompt)
		}
		if !strings.Contains(userPrompt, "上下文信息段落") {
			t.Fatalf("user prompt should include context: %q", userPrompt)
		}
		if !strings.Contains(userPrompt, "只输出改写后的 Markdown 片段") {
			t.Fatalf("user prompt should constrain response: %q", userPrompt)
		}

		response := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{{Message: chatMessage{Role: "assistant", Content: "```markdown\n改写后的片段\n```"}}},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			}{PromptTokens: 42, CompletionTokens: 256},
		}
		buf, _ := json.Marshal(response)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(buf)),
			Header:     make(http.Header),
		}, nil
	}})

	result, err := svc.RewriteSnippet(context.Background(), SnippetRewriteInput{
		Selection:   "原始片段内容",
		Instruction: "请将语气改为正式书面语",
		Context:     "上下文信息段落",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "改写后的片段" {
		t.Fatalf("unexpected rewritten content: %s", result.Content)
	}
	if result.PromptTokens != 42 || result.CompletionTokens != 256 {
		t.Fatalf("unexpected usage: %+v", result)
	}
}

func TestAIRewriteServiceRewriteSnippetValidation(t *testing.T) {
	cleanup := setupAIRewriteTestDB(t)
	defer cleanup()

	system := NewSystemSettingService(db.DB)
	svc := NewAIRewriteService(system)

	if _, err := svc.RewriteSnippet(context.Background(), SnippetRewriteInput{
		Selection:   "",
		Instruction: "说明",
	}); err == nil {
		t.Fatal("expected error when selection missing")
	}

	if _, err := svc.RewriteSnippet(context.Background(), SnippetRewriteInput{
		Selection:   "片段",
		Instruction: " ",
	}); err == nil {
		t.Fatal("expected error when instruction missing")
	}
}

func TestNormalizeAIContent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "plain text", input: "保持原样", expect: "保持原样"},
		{name: "markdown fence", input: "```markdown\n段落内容\n```", expect: "段落内容"},
		{name: "plain fence", input: "```\n段落内容\n```", expect: "段落内容"},
		{name: "windows newline", input: "```markdown\r\n段落内容\r\n```", expect: "段落内容"},
		{name: "missing closing fence", input: "```markdown\n段落内容", expect: "段落内容"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeAIContent(tc.input)
			if got != tc.expect {
				t.Fatalf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}
