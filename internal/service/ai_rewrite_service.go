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
	defaultSnippetRewriteMaxTokens  = 1024
	maxSnippetSelectionRuneCount    = 4000
	maxSnippetContextRuneCount      = 8000
)

// ErrOptimizationEmpty 表示模型未返回可用内容。
var ErrOptimizationEmpty = errors.New("ai full optimization returned empty content")

// ErrSnippetRewriteEmpty 表示片段改写未返回内容。
var ErrSnippetRewriteEmpty = errors.New("ai snippet rewrite returned empty content")

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

// SnippetRewriteInput 描述片段级改写所需的上下文。
type SnippetRewriteInput struct {
	Selection   string
	Instruction string
	Context     string
	MaxTokens   int
}

// SnippetRewriteResult 返回局部改写后的 Markdown 片段及用量信息。
type SnippetRewriteResult struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
}

// SnippetRewriter 定义局部片段改写的能力。
type SnippetRewriter interface {
	RewriteSnippet(ctx context.Context, input SnippetRewriteInput) (SnippetRewriteResult, error)
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

	sanitizedContent, placeholders := compressMarkdownImageURLs(content)
	segments := splitMarkdownSegments(sanitizedContent, maxOptimizationContentRuneCount)

	settings, err := s.client.settings.GetSettings()
	if err != nil {
		return ContentOptimizationResult{}, fmt.Errorf("读取系统设置失败: %w", err)
	}

	systemPrompt := strings.TrimSpace(settings.AIRewritePrompt)
	if systemPrompt == "" {
		systemPrompt = defaultRewriteSystemPrompt
	}

	totalSegments := len(segments)
	if totalSegments == 0 {
		return ContentOptimizationResult{}, ErrOptimizationEmpty
	}

	var optimizedParts []string
	var promptTokensSum, completionTokensSum int

	for idx, segment := range segments {
		userPrompt := buildOptimizationPrompt(segment, placeholders.Count() > 0, idx+1, totalSegments)
		label := fmt.Sprintf("OPTIMIZE-%d/%d", idx+1, totalSegments)
		logAIExchange(label, "prompt", userPrompt)

		result, err := s.client.callWithSettings(ctx, settings, aiChatRequest{
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
			MaxTokens:    maxTokens,
			Temperature:  defaultOptimizationTemperature,
		})
		if err != nil {
			return ContentOptimizationResult{}, err
		}

		optimized := normalizeAIContent(result.Content)
		if optimized == "" {
			return ContentOptimizationResult{}, ErrOptimizationEmpty
		}
		logAIExchange(label, "response", optimized)

		optimizedParts = append(optimizedParts, optimized)
		promptTokensSum += result.PromptTokens
		completionTokensSum += result.CompletionTokens
	}

	combined := strings.Join(optimizedParts, "\n\n")
	combined = placeholders.Restore(combined)

	return ContentOptimizationResult{
		Content:          combined,
		PromptTokens:     promptTokensSum,
		CompletionTokens: completionTokensSum,
	}, nil
}

// RewriteSnippet 使用 AI 对选中片段进行局部改写。
func (s *AIRewriteService) RewriteSnippet(ctx context.Context, input SnippetRewriteInput) (SnippetRewriteResult, error) {
	selection := strings.TrimSpace(input.Selection)
	if selection == "" {
		return SnippetRewriteResult{}, fmt.Errorf("selection is required")
	}

	instruction := strings.TrimSpace(input.Instruction)
	if instruction == "" {
		return SnippetRewriteResult{}, fmt.Errorf("instruction is required")
	}

	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultSnippetRewriteMaxTokens
	}

	trimmedSelection := truncateRunes(selection, maxSnippetSelectionRuneCount)
	context := strings.TrimSpace(input.Context)
	if context != "" {
		context = truncateRunes(context, maxSnippetContextRuneCount)
	}

	userPrompt := buildSnippetRewritePrompt(trimmedSelection, instruction, context)
	logAIExchange("SNIPPET", "prompt", userPrompt)

	settings, err := s.client.settings.GetSettings()
	if err != nil {
		return SnippetRewriteResult{}, fmt.Errorf("读取系统设置失败: %w", err)
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
		return SnippetRewriteResult{}, err
	}

	rewritten := normalizeAIContent(result.Content)
	if rewritten == "" {
		return SnippetRewriteResult{}, ErrSnippetRewriteEmpty
	}
	logAIExchange("SNIPPET", "response", rewritten)

	return SnippetRewriteResult{
		Content:          rewritten,
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
	}, nil
}

func splitMarkdownSegments(content string, limit int) []string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	if limit <= 0 {
		return []string{trimmed}
	}

	runes := []rune(content)
	if len(runes) <= limit {
		return []string{content}
	}

	var segments []string
	start := 0
	for start < len(runes) {
		end := start + limit
		if end >= len(runes) {
			segments = append(segments, string(runes[start:]))
			break
		}

		split := end
		for i := end; i > start; i-- {
			if runes[i-1] == '\n' {
				split = i
				break
			}
		}
		if split == start {
			split = end
		}

		segment := string(runes[start:split])
		if strings.TrimSpace(segment) != "" {
			segments = append(segments, segment)
		}
		start = split
	}

	return segments
}

func buildOptimizationPrompt(content string, hasImagePlaceholder bool, segmentIndex, totalSegments int) string {
	var builder strings.Builder
	builder.WriteString("文章正文（Markdown）：\n")
	if totalSegments > 1 {
		builder.WriteString(fmt.Sprintf("当前为第 %d/%d 段，请保持与全文一致的语气，仅润色该段内容。\n\n", segmentIndex, totalSegments))
	}
	if hasImagePlaceholder {
		builder.WriteString("注意：文中 image://asset-* 链接为图片占位符，请保持原始占位符不变。\n\n")
	}
	builder.WriteString(strings.TrimSpace(content))
	return builder.String()
}

func buildSnippetRewritePrompt(selection, instruction, context string) string {
	var builder strings.Builder
	builder.WriteString("请根据下列指令改写选中的文章片段，保持 Markdown 结构并直接返回修改后的片段。\n\n")
	builder.WriteString("【改写指令】\n")
	builder.WriteString(strings.TrimSpace(instruction))
	builder.WriteString("\n\n【原始片段】\n```markdown\n")
	builder.WriteString(strings.TrimSpace(selection))
	builder.WriteString("\n```")
	if trimmedContext := strings.TrimSpace(context); trimmedContext != "" {
		builder.WriteString("\n\n【上下文参考】\n```markdown\n")
		builder.WriteString(trimmedContext)
		builder.WriteString("\n```")
	}
	builder.WriteString("\n\n请只输出改写后的 Markdown 片段，不要附加任何说明。")
	return builder.String()
}

func normalizeAIContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ReplaceAll(trimmed, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	cleaned := stripCodeFence(normalized)
	return strings.TrimSpace(cleaned)
}

func stripCodeFence(content string) string {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) < 3 || !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	body := trimmed[3:]
	var remainder string
	if len(body) == 0 {
		return ""
	}

	if body[0] == '\n' {
		remainder = body[1:]
	} else {
		newline := strings.IndexByte(body, '\n')
		if newline == -1 {
			return strings.TrimSpace(body)
		}
		remainder = body[newline+1:]
	}

	lastFence := strings.LastIndex(remainder, "```")
	if lastFence == -1 {
		return strings.TrimSpace(remainder)
	}

	if suffix := strings.TrimSpace(remainder[lastFence:]); suffix != "```" {
		return strings.TrimSpace(remainder)
	}

	return strings.TrimSpace(remainder[:lastFence])
}
