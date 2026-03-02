package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
)

func TestPostTemplateCRUDHandlers(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tag := db.Tag{Name: "模板标签"}
	if err := db.DB.Create(&tag).Error; err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	createPayload := map[string]any{
		"name":         "每周模板",
		"description":  "固定结构",
		"content":      "# {{title}}\n今天 {{date}}",
		"summary":      "默认摘要",
		"visibility":   db.PostVisibilityUnlisted,
		"cover_url":    "https://example.com/t-cover.jpg",
		"cover_width":  1200,
		"cover_height": 630,
		"tag_ids":      []uint{tag.ID},
	}

	body, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/post-templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	api.CreatePostTemplate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var createResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	templateData, ok := createResp["template"].(map[string]any)
	if !ok {
		t.Fatalf("expected template object in create response")
	}
	templateID := uint(templateData["ID"].(float64))

	listReq := httptest.NewRequest(http.MethodGet, "/admin/api/post-templates?keyword=每周", nil)
	listW := httptest.NewRecorder()
	listC, _ := gin.CreateTestContext(listW)
	listC.Request = listReq
	api.ListPostTemplates(listC)
	if listW.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d", listW.Code)
	}

	updatePayload := map[string]any{
		"name":       "每周模板-v2",
		"content":    "# 新标题\n正文",
		"summary":    "更新摘要",
		"visibility": db.PostVisibilityPublic,
		"tag_ids":    []uint{},
	}
	updateBody, _ := json.Marshal(updatePayload)
	updateReq := httptest.NewRequest(http.MethodPut, "/admin/api/post-templates/"+strconv.Itoa(int(templateID)), bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	updateC, _ := gin.CreateTestContext(updateW)
	updateC.Request = updateReq
	updateC.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(templateID))}}
	api.UpdatePostTemplate(updateC)
	if updateW.Code != http.StatusOK {
		t.Fatalf("expected update status 200, got %d", updateW.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/admin/api/post-templates/"+strconv.Itoa(int(templateID)), nil)
	deleteW := httptest.NewRecorder()
	deleteC, _ := gin.CreateTestContext(deleteW)
	deleteC.Request = deleteReq
	deleteC.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(templateID))}}
	api.DeletePostTemplate(deleteC)
	if deleteW.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d", deleteW.Code)
	}
}

func TestCreatePostFromTemplateHandler(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	template := db.PostTemplate{
		Name:       "日报模板",
		Content:    "# {{title}}\n日期 {{date}}\n时间 {{datetime}}",
		Summary:    "默认摘要",
		Visibility: db.PostVisibilityPublic,
	}
	if err := db.DB.Create(&template).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}

	payload := map[string]any{
		"template_id": template.ID,
		"title":       "日报 2026-03-01",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/posts/from-template", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	api.CreatePostFromTemplate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	postData, ok := resp["post"].(map[string]any)
	if !ok {
		t.Fatalf("expected post object in response")
	}
	if postData["SourceTemplateID"] == nil {
		t.Fatalf("expected source template id in created post")
	}

	var persisted db.Post
	if err := db.DB.First(&persisted, uint(postData["ID"].(float64))).Error; err != nil {
		t.Fatalf("load created post: %v", err)
	}
	if persisted.SourceTemplateID == nil || *persisted.SourceTemplateID != template.ID {
		t.Fatalf("expected persisted source template id %d", template.ID)
	}

	var refreshedTemplate db.PostTemplate
	if err := db.DB.First(&refreshedTemplate, template.ID).Error; err != nil {
		t.Fatalf("reload template: %v", err)
	}
	if refreshedTemplate.UsageCount != 1 {
		t.Fatalf("expected usage_count 1, got %d", refreshedTemplate.UsageCount)
	}
	if refreshedTemplate.LastUsedAt == nil || refreshedTemplate.LastUsedAt.Before(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected last_used_at set to valid recent time")
	}
}
