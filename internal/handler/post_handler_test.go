package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeSummaryGenerator struct {
	summary          string
	err              error
	calls            int
	promptTokens     int
	completionTokens int
}

func (f *fakeSummaryGenerator) GenerateSummary(ctx context.Context, input service.SummaryInput) (service.SummaryResult, error) {
	f.calls++
	if f.err != nil {
		return service.SummaryResult{}, f.err
	}
	return service.SummaryResult{
		Summary:          f.summary,
		PromptTokens:     f.promptTokens,
		CompletionTokens: f.completionTokens,
	}, nil
}

type fakeContentOptimizer struct {
	content          string
	err              error
	calls            int
	promptTokens     int
	completionTokens int
}

func (f *fakeContentOptimizer) OptimizeContent(ctx context.Context, input service.ContentOptimizationInput) (service.ContentOptimizationResult, error) {
	f.calls++
	if f.err != nil {
		return service.ContentOptimizationResult{}, f.err
	}
	return service.ContentOptimizationResult{
		Content:          f.content,
		PromptTokens:     f.promptTokens,
		CompletionTokens: f.completionTokens,
	}, nil
}

type fakeSnippetRewriter struct {
	content          string
	err              error
	calls            int
	promptTokens     int
	completionTokens int
	lastInput        service.SnippetRewriteInput
}

func (f *fakeSnippetRewriter) RewriteSnippet(ctx context.Context, input service.SnippetRewriteInput) (service.SnippetRewriteResult, error) {
	f.calls++
	f.lastInput = input
	if f.err != nil {
		return service.SnippetRewriteResult{}, f.err
	}
	return service.SnippetRewriteResult{
		Content:          f.content,
		PromptTokens:     f.promptTokens,
		CompletionTokens: f.completionTokens,
	}, nil
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func setupTestDB(t *testing.T) (*API, func()) {
	t.Helper()

	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.User{}, &db.Post{}, &db.Tag{}, &db.Page{}, &db.ProfileContact{}, &db.SystemSetting{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	if err := gdb.Create(&db.User{Username: "tester", Password: "hashed"}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	db.DB = gdb

	return NewAPI(db.DB, "web/static/uploads", "/static/uploads"), func() {
		sqlDB, err := db.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestCreatePostWithTags(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tag := db.Tag{Name: "Go"}
	if err := db.DB.Create(&tag).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	payload := map[string]any{
		"title":        "Test Post",
		"content":      "# Test Post\nContent",
		"summary":      "Summary",
		"status":       "draft",
		"tag_ids":      []uint{tag.ID},
		"cover_url":    "https://images.unsplash.com/photo-1500530855697-b586d89ba3ee",
		"cover_width":  1200,
		"cover_height": 800,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.CreatePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var created db.Post
	if err := db.DB.Preload("Tags").First(&created).Error; err != nil {
		t.Fatalf("failed to load created post: %v", err)
	}

	if created.Title != "Test Post" {
		t.Fatalf("unexpected title: %s", created.Title)
	}

	if len(created.Tags) != 1 || created.Tags[0].ID != tag.ID {
		t.Fatalf("expected associated tag with ID %d", tag.ID)
	}
}

func TestCreatePostRejectsUnknownTags(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	payload := map[string]any{
		"title":        "Test Post",
		"content":      "# Test Post\nContent",
		"summary":      "Summary",
		"status":       "draft",
		"tag_ids":      []uint{99},
		"cover_url":    "https://images.unsplash.com/photo-1500530855697-b586d89ba3ee",
		"cover_width":  1200,
		"cover_height": 800,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.CreatePost(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestPublishPostDoesNotAutoGenerateSummary(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	stub := &fakeSummaryGenerator{summary: "AI 生成的摘要"}
	api.summaries = stub

	payload := map[string]any{
		"title":        "AI Test",
		"content":      "这是正文内容。",
		"summary":      "",
		"tag_ids":      []uint{},
		"cover_url":    "https://images.unsplash.com/photo-1500530855697-b586d89ba3ee",
		"cover_width":  1200,
		"cover_height": 800,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.CreatePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if stub.calls != 0 {
		t.Fatalf("expected summary generator not to run when saving draft, got %d", stub.calls)
	}

	var createResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	postData, ok := createResp["post"].(map[string]any)
	if !ok {
		t.Fatalf("expected create response to include post object")
	}
	postID := uint(postData["ID"].(float64))

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	publishReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/api/posts/%d/publish", postID), nil)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", postID)}}
	c.Request = publishReq

	api.PublishPost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected publish status 200, got %d", w.Code)
	}

	if stub.calls != 0 {
		t.Fatalf("expected summary generator not to run automatically, got %d", stub.calls)
	}

	var publishResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &publishResp); err != nil {
		t.Fatalf("failed to decode publish response: %v", err)
	}

	var created db.Post
	if err := db.DB.First(&created, postID).Error; err != nil {
		t.Fatalf("failed to load created post: %v", err)
	}
	if created.Status != "published" {
		t.Fatalf("expected post status published, got %s", created.Status)
	}
	if created.Summary != "" {
		t.Fatalf("expected summary to stay empty, got %q", created.Summary)
	}
	if created.LatestPublicationID == nil {
		t.Fatalf("expected latest publication id to be stored")
	}

	publicationData, ok := publishResp["publication"].(map[string]any)
	if !ok {
		t.Fatalf("expected publish response to include publication")
	}
	if summary, _ := publicationData["Summary"].(string); summary != "" {
		t.Fatalf("expected publication summary to stay empty, got %q", summary)
	}
}

func TestPublishPostDoesNotAutoGenerateSummaryWhenFailed(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	stub := &fakeSummaryGenerator{err: errors.New("network error")}
	api.summaries = stub

	payload := map[string]any{
		"title":        "AI Failure",
		"content":      "正文内容",
		"summary":      "",
		"tag_ids":      []uint{},
		"cover_url":    "https://images.unsplash.com/photo-1500530855697-b586d89ba3ee",
		"cover_width":  1200,
		"cover_height": 800,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.CreatePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if stub.calls != 0 {
		t.Fatalf("expected summary generator not to run when saving draft, got %d", stub.calls)
	}

	var createResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	postData, ok := createResp["post"].(map[string]any)
	if !ok {
		t.Fatalf("expected create response to include post object")
	}
	postID := uint(postData["ID"].(float64))

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	publishReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/api/posts/%d/publish", postID), nil)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", postID)}}
	c.Request = publishReq

	api.PublishPost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected publish status 200, got %d", w.Code)
	}

	if stub.calls != 0 {
		t.Fatalf("expected summary generator not to run automatically, got %d", stub.calls)
	}

	var created db.Post
	if err := db.DB.First(&created, postID).Error; err != nil {
		t.Fatalf("failed to load post: %v", err)
	}
	if created.Summary != "" {
		t.Fatalf("expected summary to remain empty on failure")
	}
}

func TestUpdatePostReplacesTags(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	originalTag := db.Tag{Name: "Go"}
	if err := db.DB.Create(&originalTag).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	replacementTag := db.Tag{Name: "Gin"}
	if err := db.DB.Create(&replacementTag).Error; err != nil {
		t.Fatalf("failed to seed replacement tag: %v", err)
	}

	post := db.Post{
		Content:     "# Original\nOriginal content",
		Status:      "draft",
		UserID:      1,
		CoverURL:    "https://images.unsplash.com/photo-1498050108023-c5249f4df085",
		CoverWidth:  1280,
		CoverHeight: 720,
	}

	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	if err := db.DB.Model(&post).Association("Tags").Append(&originalTag); err != nil {
		t.Fatalf("failed to associate original tag: %v", err)
	}

	payload := map[string]any{
		"title":        "Updated",
		"content":      "# Updated\nUpdated content",
		"summary":      "Updated summary",
		"status":       "published",
		"tag_ids":      []uint{replacementTag.ID},
		"cover_url":    "https://images.unsplash.com/photo-1523475472560-d2df97ec485c",
		"cover_width":  1440,
		"cover_height": 960,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/posts/"+strconv.Itoa(int(post.ID)), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(post.ID))}}

	api.UpdatePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var updated db.Post
	if err := db.DB.Preload("Tags").First(&updated, post.ID).Error; err != nil {
		t.Fatalf("failed to load updated post: %v", err)
	}

	if updated.Title != "Updated" {
		t.Fatalf("unexpected title: %s", updated.Title)
	}

	if len(updated.Tags) != 1 || updated.Tags[0].ID != replacementTag.ID {
		t.Fatalf("expected associated tag with ID %d", replacementTag.ID)
	}
}

func TestUpdatePostRejectsUnknownTags(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	post := db.Post{
		Content:     "# Original\nOriginal content",
		Status:      "draft",
		UserID:      1,
		CoverURL:    "https://images.unsplash.com/photo-1498050108023-c5249f4df085",
		CoverWidth:  1280,
		CoverHeight: 720,
	}

	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	payload := map[string]any{
		"title":        "Updated",
		"content":      "# Updated\nUpdated content",
		"summary":      "Updated summary",
		"status":       "published",
		"tag_ids":      []uint{123},
		"cover_url":    "https://images.unsplash.com/photo-1523475472560-d2df97ec485c",
		"cover_width":  1440,
		"cover_height": 960,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/posts/"+strconv.Itoa(int(post.ID)), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(post.ID))}}

	api.UpdatePost(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestUpdatePostClearsTagsWhenEmpty(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tag := db.Tag{Name: "Go"}
	if err := db.DB.Create(&tag).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	post := db.Post{
		Content:     "# Original\nOriginal content",
		Status:      "draft",
		UserID:      1,
		CoverURL:    "https://images.unsplash.com/photo-1498050108023-c5249f4df085",
		CoverWidth:  1280,
		CoverHeight: 720,
	}

	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	if err := db.DB.Model(&post).Association("Tags").Append(&tag); err != nil {
		t.Fatalf("failed to associate tag: %v", err)
	}

	payload := map[string]any{
		"title":        "Updated",
		"content":      "# Updated\nUpdated content",
		"summary":      "Updated summary",
		"status":       "published",
		"tag_ids":      []uint{},
		"cover_url":    "https://images.unsplash.com/photo-1500530855697-b586d89ba3ee",
		"cover_width":  1200,
		"cover_height": 800,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/posts/"+strconv.Itoa(int(post.ID)), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(post.ID))}}

	api.UpdatePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var updated db.Post
	if err := db.DB.Preload("Tags").First(&updated, post.ID).Error; err != nil {
		t.Fatalf("failed to load updated post: %v", err)
	}

	if len(updated.Tags) != 0 {
		t.Fatalf("expected no tags, found %d", len(updated.Tags))
	}
}

func TestDeletePost(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	post := db.Post{
		Content:     "# Delete Me\nContent",
		Status:      "draft",
		UserID:      1,
		CoverURL:    "https://images.unsplash.com/photo-1498050108023-c5249f4df085",
		CoverWidth:  1280,
		CoverHeight: 720,
	}

	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/posts/"+strconv.Itoa(int(post.ID)), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(post.ID))}}

	api.DeletePost(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var count int64
	db.DB.Model(&db.Post{}).Where("id = ?", post.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected post to be deleted, still found %d records", count)
	}
}

func TestGeneratePostSummarySuccess(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	stub := &fakeSummaryGenerator{
		summary:          "这是一个精炼摘要",
		promptTokens:     120,
		completionTokens: 36,
	}
	api.summaries = stub

	payload := map[string]any{
		"title":   "Go 并发模式",
		"content": "Goroutine 和 channel 的组合...",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/summary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.GeneratePostSummary(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Summary string `json:"summary"`
		Usage   struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Summary != stub.summary {
		t.Fatalf("unexpected summary: %s", resp.Summary)
	}

	if resp.Usage.PromptTokens != stub.promptTokens {
		t.Fatalf("unexpected prompt tokens: %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != stub.completionTokens {
		t.Fatalf("unexpected completion tokens: %d", resp.Usage.CompletionTokens)
	}
}

func TestGeneratePostSummaryRequiresContent(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	api.summaries = &fakeSummaryGenerator{summary: "不会触发"}

	payload := map[string]any{
		"title":   " ",
		"content": "",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/summary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.GeneratePostSummary(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "请至少提供文章标题或正文内容" {
		t.Fatalf("unexpected error message: %v", resp["error"])
	}
}

func TestGeneratePostSummaryMissingAPIKey(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	api.summaries = &fakeSummaryGenerator{err: service.ErrAIAPIKeyMissing}

	payload := map[string]any{
		"title":   "测试",
		"content": "内容",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/summary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.GeneratePostSummary(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "请在系统设置中配置有效的 AI API Key 后再试" {
		t.Fatalf("unexpected error message: %v", resp["error"])
	}
}

func TestOptimizePostContentSuccess(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	stub := &fakeContentOptimizer{
		content:          "优化后的 Markdown 内容",
		promptTokens:     256,
		completionTokens: 768,
	}
	api.optimizer = stub

	payload := map[string]any{
		"title":   "测试标题",
		"content": "原始 Markdown 内容",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/optimize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.OptimizePostContent(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Optimized string `json:"optimized_content"`
		Usage     struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Optimized != stub.content {
		t.Fatalf("unexpected optimized content: %s", resp.Optimized)
	}
	if resp.Usage.PromptTokens != stub.promptTokens {
		t.Fatalf("unexpected prompt tokens: %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != stub.completionTokens {
		t.Fatalf("unexpected completion tokens: %d", resp.Usage.CompletionTokens)
	}
}

func TestOptimizePostContentRequiresContent(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	api.optimizer = &fakeContentOptimizer{content: "不会触发"}

	payload := map[string]any{
		"title":   "标题",
		"content": "   ",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/optimize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.OptimizePostContent(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "请先填写文章正文后再尝试全文优化" {
		t.Fatalf("unexpected error message: %v", resp["error"])
	}
}

func TestRewritePostSelectionSuccess(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	stub := &fakeSnippetRewriter{
		content:          "改写后的段落",
		promptTokens:     88,
		completionTokens: 144,
	}
	api.snippetRewriter = stub

	payload := map[string]any{
		"selection":   "原始段落",
		"instruction": "请改成更具说服力的语气",
		"context":     "上下文说明",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.RewritePostSelection(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Content string `json:"content"`
		Usage   struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Content != stub.content {
		t.Fatalf("unexpected rewritten content: %s", resp.Content)
	}
	if resp.Usage.PromptTokens != stub.promptTokens || resp.Usage.CompletionTokens != stub.completionTokens {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}

	if stub.calls != 1 {
		t.Fatalf("expected rewriter to be called once, got %d", stub.calls)
	}
	if stub.lastInput.Selection != "原始段落" || stub.lastInput.Instruction != "请改成更具说服力的语气" {
		t.Fatalf("unexpected input: %+v", stub.lastInput)
	}
}

func TestRewritePostSelectionRequiresSelection(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	api.snippetRewriter = &fakeSnippetRewriter{content: "不会触发"}

	payload := map[string]any{
		"selection":   " ",
		"instruction": "请改成正式语气",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.RewritePostSelection(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "请先选择需要改写的段落后再试" {
		t.Fatalf("unexpected error message: %v", resp["error"])
	}
}

func TestRewritePostSelectionRequiresInstruction(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	api.snippetRewriter = &fakeSnippetRewriter{content: "不会触发"}

	payload := map[string]any{
		"selection":   "原文",
		"instruction": "",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.RewritePostSelection(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "请输入改写指令" {
		t.Fatalf("unexpected error message: %v", resp["error"])
	}
}

func TestRewritePostSelectionMissingAPIKey(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	api.snippetRewriter = &fakeSnippetRewriter{err: service.ErrAIAPIKeyMissing}

	payload := map[string]any{
		"selection":   "原文",
		"instruction": "请改成正式语气",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.RewritePostSelection(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "请在系统设置中配置有效的 AI API Key 后再试" {
		t.Fatalf("unexpected error message: %v", resp["error"])
	}
}

func TestOptimizePostContentMissingAPIKey(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	api.optimizer = &fakeContentOptimizer{err: service.ErrAIAPIKeyMissing}

	payload := map[string]any{
		"title":   "标题",
		"content": "正文",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/optimize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.OptimizePostContent(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "请在系统设置中配置有效的 AI API Key 后再试" {
		t.Fatalf("unexpected error message: %v", resp["error"])
	}
}
