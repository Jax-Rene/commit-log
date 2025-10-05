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
	defaultOpenAISummaryModel   = "gpt-4o-mini"
	defaultDeepSeekSummaryModel = "deepseek-chat"
	defaultSummaryMaxTokens     = 160
	defaultSummaryTemperature   = 0.2
	maxSummaryContentRuneCount  = 4000
)

// AISummaryService 基于大模型接口生成文章摘要。
type AISummaryService struct {
	settings        *SystemSettingService
	http            httpDoer
	openAIBaseURL   string
	openAIModel     string
	deepSeekBaseURL string
	deepSeekModel   string
}

// NewAISummaryService 构造默认的 AISummaryService。
func NewAISummaryService(settings *SystemSettingService) *AISummaryService {
	return &AISummaryService{
		settings:        settings,
		http:            &http.Client{Timeout: 20 * time.Second},
		openAIBaseURL:   "https://api.openai.com/v1",
		openAIModel:     defaultOpenAISummaryModel,
		deepSeekBaseURL: "https://api.deepseek.com/v1",
		deepSeekModel:   defaultDeepSeekSummaryModel,
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

// SetBaseURL 兼容旧方法，等价于 SetOpenAIBaseURL。
func (s *AISummaryService) SetBaseURL(base string) {
	s.SetOpenAIBaseURL(base)
}

// SetOpenAIBaseURL 覆盖默认的 OpenAI API 地址。
func (s *AISummaryService) SetOpenAIBaseURL(base string) {
	s.openAIBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
}

// SetDeepSeekBaseURL 覆盖默认的 DeepSeek API 地址。
func (s *AISummaryService) SetDeepSeekBaseURL(base string) {
	s.deepSeekBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
}

// SetModel 兼容旧方法，等价于 SetOpenAIModel。
func (s *AISummaryService) SetModel(model string) {
	s.SetOpenAIModel(model)
}

// SetOpenAIModel 指定 OpenAI 摘要所使用的模型名称。
func (s *AISummaryService) SetOpenAIModel(model string) {
	model = strings.TrimSpace(model)
	if model != "" {
		s.openAIModel = model
	}
}

// SetDeepSeekModel 指定 DeepSeek 摘要所使用的模型名称。
func (s *AISummaryService) SetDeepSeekModel(model string) {
	model = strings.TrimSpace(model)
	if model != "" {
		s.deepSeekModel = model
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

// GenerateSummary 调用当前配置的 AI 平台生成文章摘要，当未配置 API Key 时返回 ErrAIAPIKeyMissing。
func (s *AISummaryService) GenerateSummary(ctx context.Context, input SummaryInput) (SummaryResult, error) {
	settings, err := s.settings.GetSettings()
	if err != nil {
		return SummaryResult{}, fmt.Errorf("读取系统设置失败: %w", err)
	}

	provider := normalizeAIProvider(settings.AIProvider)
	if provider == "" {
		provider = AIProviderOpenAI
	}

	var (
		apiKey string
		base   string
		model  string
		label  string
	)

	switch provider {
	case AIProviderDeepSeek:
		apiKey = strings.TrimSpace(settings.DeepSeekAPIKey)
		base = s.deepSeekBaseURL
		if strings.TrimSpace(base) == "" {
			base = "https://api.deepseek.com/v1"
		}
		model = s.deepSeekModel
		if strings.TrimSpace(model) == "" {
			model = defaultDeepSeekSummaryModel
		}
		label = "DeepSeek"
	default:
		apiKey = strings.TrimSpace(settings.OpenAIAPIKey)
		base = s.openAIBaseURL
		if strings.TrimSpace(base) == "" {
			base = "https://api.openai.com/v1"
		}
		model = s.openAIModel
		if strings.TrimSpace(model) == "" {
			model = defaultOpenAISummaryModel
		}
		label = "OpenAI"
	}

	if apiKey == "" {
		return SummaryResult{}, ErrAIAPIKeyMissing
	}

	client := s.http
	if client == nil {
		client = http.DefaultClient
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
		return SummaryResult{}, fmt.Errorf("创建 %s 请求失败: %w", label, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "commitlog-ai-summary/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return SummaryResult{}, fmt.Errorf("请求 %s 接口失败: %w", label, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return SummaryResult{}, fmt.Errorf("读取 %s 响应失败: %w", label, err)
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return SummaryResult{}, fmt.Errorf("解析 %s 响应失败: %w", label, err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		errMsg := strings.TrimSpace(completion.Error.Message)
		if errMsg == "" {
			errMsg = strings.TrimSpace(string(respBody))
		}
		if errMsg == "" {
			errMsg = resp.Status
		}
		return SummaryResult{}, fmt.Errorf("%s 摘要接口返回错误：%s", label, errMsg)
	}

	if len(completion.Choices) == 0 {
		return SummaryResult{}, fmt.Errorf("%s 摘要接口未返回结果", label)
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
