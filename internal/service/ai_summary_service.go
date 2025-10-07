package service

import (
	"context"
	"strings"
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
	client *aiChatClient
}

// NewAISummaryService 构造默认的 AISummaryService。
func NewAISummaryService(settings *SystemSettingService) *AISummaryService {
	return &AISummaryService{
		client: newAIChatClient(settings, defaultOpenAISummaryModel, defaultDeepSeekSummaryModel),
	}
}

// SetHTTPClient 覆盖默认 HTTP 客户端，主要用于测试。
func (s *AISummaryService) SetHTTPClient(client httpDoer) {
	s.client.SetHTTPClient(client)
}

// SetBaseURL 兼容旧方法，等价于 SetOpenAIBaseURL。
func (s *AISummaryService) SetBaseURL(base string) {
	s.SetOpenAIBaseURL(base)
}

// SetOpenAIBaseURL 覆盖默认的 OpenAI API 地址。
func (s *AISummaryService) SetOpenAIBaseURL(base string) {
	s.client.SetOpenAIBaseURL(base)
}

// SetDeepSeekBaseURL 覆盖默认的 DeepSeek API 地址。
func (s *AISummaryService) SetDeepSeekBaseURL(base string) {
	s.client.SetDeepSeekBaseURL(base)
}

// SetModel 兼容旧方法，等价于 SetOpenAIModel。
func (s *AISummaryService) SetModel(model string) {
	s.SetOpenAIModel(model)
}

// SetOpenAIModel 指定 OpenAI 摘要所使用的模型名称。
func (s *AISummaryService) SetOpenAIModel(model string) {
	s.client.SetOpenAIModel(model)
}

// SetDeepSeekModel 指定 DeepSeek 摘要所使用的模型名称。
func (s *AISummaryService) SetDeepSeekModel(model string) {
	s.client.SetDeepSeekModel(model)
}

// GenerateSummary 调用当前配置的 AI 平台生成文章摘要，当未配置 API Key 时返回 ErrAIAPIKeyMissing。
func (s *AISummaryService) GenerateSummary(ctx context.Context, input SummaryInput) (SummaryResult, error) {
	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultSummaryMaxTokens
	}

	contentSnippet := truncateRunes(input.Content, maxSummaryContentRuneCount)
	userPrompt := buildSummaryPrompt(input.Title, contentSnippet)

	result, err := s.client.call(ctx, aiChatRequest{
		SystemPrompt: "你是一名中文博客编辑，需要在 80 个汉字以内给出精炼、有吸引力的摘要。摘要需突出观点，不要使用项目符号。",
		UserPrompt:   userPrompt,
		MaxTokens:    maxTokens,
		Temperature:  defaultSummaryTemperature,
	})
	if err != nil {
		return SummaryResult{}, err
	}

	summary := strings.TrimSpace(result.Content)
	return SummaryResult{
		Summary:          summary,
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
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
