package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
)

func TestCreateTagDuplicateName(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	existing := db.Tag{Name: "Go"}
	if err := db.DB.Create(&existing).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	payload := map[string]any{"name": "Go"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/tags", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.CreateTag(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateTagDuplicateName(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tagA := db.Tag{Name: "Go"}
	tagB := db.Tag{Name: "Gin"}

	if err := db.DB.Create(&tagA).Error; err != nil {
		t.Fatalf("failed to seed tagA: %v", err)
	}
	if err := db.DB.Create(&tagB).Error; err != nil {
		t.Fatalf("failed to seed tagB: %v", err)
	}

	payload := map[string]any{"name": "Go"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/tags/"+strconv.Itoa(int(tagB.ID)), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(tagB.ID))}}

	api.UpdateTag(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteTagBlockedWhenInUse(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tag := db.Tag{Name: "Go"}
	if err := db.DB.Create(&tag).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	post := db.Post{Title: "Test", Content: "Content", Status: "draft", UserID: 1}
	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	if err := db.DB.Model(&post).Association("Tags").Append(&tag); err != nil {
		t.Fatalf("failed to associate tag: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/tags/"+strconv.Itoa(int(tag.ID)), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(tag.ID))}}

	api.DeleteTag(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteTagSuccess(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tag := db.Tag{Name: "Go"}
	if err := db.DB.Create(&tag).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/tags/"+strconv.Itoa(int(tag.ID)), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(tag.ID))}}

	api.DeleteTag(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var count int64
	db.DB.Model(&db.Tag{}).Where("id = ?", tag.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected tag to be deleted, still found %d records", count)
	}
}

func TestGetTagsReturnsSortedList(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tags := []db.Tag{{Name: "Zed"}, {Name: "Alpha"}}
	if err := db.DB.Create(&tags).Error; err != nil {
		t.Fatalf("failed to seed tags: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/tags", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.GetTags(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Tags []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		}
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(resp.Tags))
	}

	if resp.Tags[0].Name != "Alpha" || resp.Tags[1].Name != "Zed" {
		t.Fatalf("expected tags to be sorted ascending: %v", resp.Tags)
	}
}
