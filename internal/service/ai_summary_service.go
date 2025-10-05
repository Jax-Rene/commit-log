package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SummaryInput 描述生成文章摘要所需的上下文。
type SummaryInput struct {
	Title   string
	Content string
	// MaxTokens 控制模型输出上限，0 表示使用默认值。
	MaxTokens int
}

// SummaryResult 返回模型生成的摘要及少量元数据。
type SummaryResult struct {
	Summary          string
	PromptTokens     int
	CompletionTokens int
}

// SummaryGenerator 定义摘要生成能力，便于在业务层注入不同实现。
type SummaryGenerator interface {
	GenerateSummary(ctx context.Context, input SummaryInput) (SummaryResult, error)
}

const (
	defaultSummaryModel        = "gpt-4o-mini"
	defaultSummaryMaxTokens    = 160
	defaultSummaryTemperature  = 0.2
	maxSummaryContentRuneCount = 4000
)

// AISummaryService 基于 OpenAI Chat Completions 生成文章摘要。
type AISummaryService struct {
	settings *SystemSettingService
	http     httpDoer
	baseURL  string
	model    string
}

// NewAISummaryService 构造默认的 AISummaryService。
func NewAISummaryService(settings *SystemSettingService) *AISummaryService {
	return &AISummaryService{
		settings: settings,
		http:     &http.Client{Timeout: 20 * time.Second},
		baseURL:  "https://api.openai.com/v1",
		model:    defaultSummaryModel,
	}
}

// SetHTTPClient 覆盖默认 HTTP 客户端，主要用于测试。
func (s *AISummaryService) SetHTTPClient(client httpDoer) {
	if client == nil {
		s.http = &http.Client{Timeout: 20 * time.Second}
		return
	}
	s.http = client
}

// SetBaseURL 覆盖默认的 OpenAI API 地址，支持自定义代理或测试桩。
func (s *AISummaryService) SetBaseURL(base string) {
	s.baseURL = strings.TrimRight(strings.TrimSpace(base), "/")
}

// SetModel 指定摘要使用的模型名称。
func (s *AISummaryService) SetModel(model string) {
	model = strings.TrimSpace(model)
	if model != "" {
		s.model = model
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// GenerateSummary 调用 OpenAI 接口生成文章摘要，当未配置 API Key 时返回 ErrOpenAIAPIKeyMissing。
func (s *AISummaryService) GenerateSummary(ctx context.Context, input SummaryInput) (SummaryResult, error) {
	settings, err := s.settings.GetSettings()
	if err != nil {
		return SummaryResult{}, fmt.Errorf("读取系统设置失败: %w", err)
	}
	apiKey := strings.TrimSpace(settings.OpenAIAPIKey)
	if apiKey == "" {
		return SummaryResult{}, ErrOpenAIAPIKeyMissing
	}

	client := s.http
	if client == nil {
		client = http.DefaultClient
	}

	base := s.baseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}

	model := s.model
	if strings.TrimSpace(model) == "" {
		model = defaultSummaryModel
	}

	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultSummaryMaxTokens
	}

	contentSnippet := truncateRunes(input.Content, maxSummaryContentRuneCount)
	userPrompt := buildSummaryPrompt(input.Title, contentSnippet)

	payload := chatCompletionRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: "你是一名中文博客编辑，需要在 80 个汉字以内给出精炼、有吸引力的摘要。摘要需突出观点，不要使用项目符号。"},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   maxTokens,
		Temperature: defaultSummaryTemperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return SummaryResult{}, fmt.Errorf("构造请求失败: %w", err)
	}

	endpoint := base + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return SummaryResult{}, fmt.Errorf("创建 OpenAI 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "commitlog-ai-summary/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return SummaryResult{}, fmt.Errorf("请求 OpenAI 接口失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return SummaryResult{}, fmt.Errorf("读取 OpenAI 响应失败: %w", err)
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return SummaryResult{}, fmt.Errorf("解析 OpenAI 响应失败: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		errMsg := strings.TrimSpace(completion.Error.Message)
		if errMsg == "" {
			errMsg = strings.TrimSpace(string(respBody))
		}
		if errMsg == "" {
			errMsg = resp.Status
		}
		return SummaryResult{}, fmt.Errorf("OpenAI 摘要接口返回错误：%s", errMsg)
	}

	if len(completion.Choices) == 0 {
		return SummaryResult{}, fmt.Errorf("OpenAI 摘要接口未返回结果")
	}

	summary := strings.TrimSpace(completion.Choices[0].Message.Content)
	return SummaryResult{
		Summary:          summary,
		PromptTokens:     completion.Usage.PromptTokens,
		CompletionTokens: completion.Usage.CompletionTokens,
	}, nil
}

func buildSummaryPrompt(title, content string) string {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	var builder strings.Builder
	if title != "" {
		builder.WriteString("标题：")
		builder.WriteString(title)
		builder.WriteString("\n")
	}
	if content != "" {
		builder.WriteString("正文：\n")
		builder.WriteString(content)
	}
	return builder.String()
}

func truncateRunes(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	return string(runes[:limit])
}
