package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	defaultOpenAIRewriteModel       = "gpt-4o-mini"
	defaultDeepSeekRewriteModel     = "deepseek-chat"
	defaultOptimizationMaxTokens    = 4096
	defaultOptimizationTemperature  = 0.35
	maxOptimizationContentRuneCount = 16000
)

// ErrOptimizationEmpty 表示模型未返回可用内容。
var ErrOptimizationEmpty = errors.New("ai full optimization returned empty content")

// ContentOptimizationInput 描述调用全文优化所需的上下文。
type ContentOptimizationInput struct {
	Content   string
	MaxTokens int
}

// ContentOptimizationResult 返回全文优化后的 Markdown 结果及用量信息。
type ContentOptimizationResult struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
}

// ContentOptimizer 定义全文优化的能力，便于在业务层注入不同实现。
type ContentOptimizer interface {
	OptimizeContent(ctx context.Context, input ContentOptimizationInput) (ContentOptimizationResult, error)
}

// AIRewriteService 基于大模型接口对文章进行全文优化。
type AIRewriteService struct {
	client *aiChatClient
}

// NewAIRewriteService 构造默认的 AIRewriteService。
func NewAIRewriteService(settings *SystemSettingService) *AIRewriteService {
	return &AIRewriteService{
		client: newAIChatClient(settings, defaultOpenAIRewriteModel, defaultDeepSeekRewriteModel),
	}
}

// SetHTTPClient 覆盖默认 HTTP 客户端，主要用于测试。
func (s *AIRewriteService) SetHTTPClient(client httpDoer) {
	s.client.SetHTTPClient(client)
}

// SetOpenAIBaseURL 覆盖默认的 OpenAI API 地址。
func (s *AIRewriteService) SetOpenAIBaseURL(base string) {
	s.client.SetOpenAIBaseURL(base)
}

// SetDeepSeekBaseURL 覆盖默认的 DeepSeek API 地址。
func (s *AIRewriteService) SetDeepSeekBaseURL(base string) {
	s.client.SetDeepSeekBaseURL(base)
}

// SetOpenAIModel 指定 OpenAI 全文优化所使用的模型名称。
func (s *AIRewriteService) SetOpenAIModel(model string) {
	s.client.SetOpenAIModel(model)
}

// SetDeepSeekModel 指定 DeepSeek 全文优化所使用的模型名称。
func (s *AIRewriteService) SetDeepSeekModel(model string) {
	s.client.SetDeepSeekModel(model)
}

// OptimizeContent 调用大模型对文章 Markdown 进行整体润色优化。
func (s *AIRewriteService) OptimizeContent(ctx context.Context, input ContentOptimizationInput) (ContentOptimizationResult, error) {
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return ContentOptimizationResult{}, fmt.Errorf("content is required")
	}

	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultOptimizationMaxTokens
	}

	contentSnippet := truncateRunes(content, maxOptimizationContentRuneCount)
	userPrompt := buildOptimizationPrompt(contentSnippet)

	settings, err := s.client.settings.GetSettings()
	if err != nil {
		return ContentOptimizationResult{}, fmt.Errorf("读取系统设置失败: %w", err)
	}

	systemPrompt := strings.TrimSpace(settings.AIRewritePrompt)
	if systemPrompt == "" {
		systemPrompt = defaultRewriteSystemPrompt
	}

	result, err := s.client.callWithSettings(ctx, settings, aiChatRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    maxTokens,
		Temperature:  defaultOptimizationTemperature,
	})
	if err != nil {
		return ContentOptimizationResult{}, err
	}

	optimized := strings.TrimSpace(result.Content)
	if optimized == "" {
		return ContentOptimizationResult{}, ErrOptimizationEmpty
	}

	return ContentOptimizationResult{
		Content:          optimized,
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
	}, nil
}

func buildOptimizationPrompt(content string) string {
	var builder strings.Builder
	builder.WriteString("文章正文（Markdown）：\n")
	builder.WriteString(strings.TrimSpace(content))
	return builder.String()
}
