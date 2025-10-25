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

type aiChatRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	Temperature  float64
}

type aiChatResponse struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
}

type aiChatClient struct {
	settings             *SystemSettingService
	http                 httpDoer
	openAIBaseURL        string
	openAIModel          string
	deepSeekBaseURL      string
	deepSeekModel        string
	defaultOpenAIModel   string
	defaultDeepSeekModel string
}

func newAIChatClient(settings *SystemSettingService, defaultOpenAIModel, defaultDeepSeekModel string) *aiChatClient {
	return &aiChatClient{
		settings:             settings,
		http:                 &http.Client{Timeout: 180 * time.Second},
		openAIBaseURL:        "https://api.openai.com/v1",
		openAIModel:          strings.TrimSpace(defaultOpenAIModel),
		deepSeekBaseURL:      "https://api.deepseek.com/v1",
		deepSeekModel:        strings.TrimSpace(defaultDeepSeekModel),
		defaultOpenAIModel:   strings.TrimSpace(defaultOpenAIModel),
		defaultDeepSeekModel: strings.TrimSpace(defaultDeepSeekModel),
	}
}

func (c *aiChatClient) SetHTTPClient(client httpDoer) {
	if client == nil {
		c.http = &http.Client{Timeout: 20 * time.Second}
		return
	}
	c.http = client
}

func (c *aiChatClient) SetOpenAIBaseURL(base string) {
	c.openAIBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
}

func (c *aiChatClient) SetDeepSeekBaseURL(base string) {
	c.deepSeekBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
}

func (c *aiChatClient) SetOpenAIModel(model string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}
	c.openAIModel = model
}

func (c *aiChatClient) SetDeepSeekModel(model string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}
	c.deepSeekModel = model
}

func (c *aiChatClient) callWithSettings(ctx context.Context, settings SystemSettings, req aiChatRequest) (aiChatResponse, error) {
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
		base = c.deepSeekBaseURL
		if strings.TrimSpace(base) == "" {
			base = "https://api.deepseek.com/v1"
		}
		model = c.deepSeekModel
		if strings.TrimSpace(model) == "" {
			model = c.defaultDeepSeekModel
		}
		label = "DeepSeek"
	default:
		apiKey = strings.TrimSpace(settings.OpenAIAPIKey)
		base = c.openAIBaseURL
		if strings.TrimSpace(base) == "" {
			base = "https://api.openai.com/v1"
		}
		model = c.openAIModel
		if strings.TrimSpace(model) == "" {
			model = c.defaultOpenAIModel
		}
		label = "OpenAI"
	}

	if apiKey == "" {
		return aiChatResponse{}, ErrAIAPIKeyMissing
	}

	client := c.http
	if client == nil {
		client = http.DefaultClient
	}

	maxTokens := req.MaxTokens
	if maxTokens < 0 {
		maxTokens = 0
	}

	payload := chatCompletionRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: strings.TrimSpace(req.SystemPrompt)},
			{Role: "user", Content: req.UserPrompt},
		},
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return aiChatResponse{}, fmt.Errorf("构造请求失败: %w", err)
	}

	endpoint := strings.TrimRight(base, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return aiChatResponse{}, fmt.Errorf("创建 %s 请求失败: %w", label, err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "commitlog-ai/1.0")

	resp, err := client.Do(httpReq)
	if err != nil {
		return aiChatResponse{}, fmt.Errorf("请求 %s 接口失败: %w", label, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return aiChatResponse{}, fmt.Errorf("读取 %s 响应失败: %w", label, err)
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return aiChatResponse{}, fmt.Errorf("解析 %s 响应失败: %w", label, err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		errMsg := strings.TrimSpace(completion.Error.Message)
		if errMsg == "" {
			errMsg = strings.TrimSpace(string(respBody))
		}
		if errMsg == "" {
			errMsg = resp.Status
		}
		return aiChatResponse{}, fmt.Errorf("%s 接口返回错误：%s", label, errMsg)
	}

	if len(completion.Choices) == 0 {
		return aiChatResponse{}, fmt.Errorf("%s 接口未返回结果", label)
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	return aiChatResponse{
		Content:          content,
		PromptTokens:     completion.Usage.PromptTokens,
		CompletionTokens: completion.Usage.CompletionTokens,
	}, nil
}
