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

	post := db.Post{Content: "# Test\nContent", Status: "draft", UserID: 1}
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
	db.DB.Unscoped().Model(&db.Tag{}).Where("id = ?", tag.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected tag to be deleted, still found %d records", count)
	}
}

func TestGetTagsReturnsSortedList(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tags := []db.Tag{
		{Name: "Alpha", SortOrder: 1},
		{Name: "Zed", SortOrder: 0},
	}
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
			ID        uint   `json:"id"`
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		}
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(resp.Tags))
	}

	if resp.Tags[0].Name != "Zed" || resp.Tags[1].Name != "Alpha" {
		t.Fatalf("expected tags to follow sort order: %v", resp.Tags)
	}
	if resp.Tags[0].SortOrder != 0 || resp.Tags[1].SortOrder != 1 {
		t.Fatalf("expected sort_order to be returned: %v", resp.Tags)
	}
}

func TestReorderTagsSuccess(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tags := []db.Tag{
		{Name: "Go", SortOrder: 0},
		{Name: "Gin", SortOrder: 1},
		{Name: "Gorm", SortOrder: 2},
	}
	if err := db.DB.Create(&tags).Error; err != nil {
		t.Fatalf("failed to seed tags: %v", err)
	}

	payload := map[string]any{
		"ids": []uint{tags[2].ID, tags[0].ID, tags[1].ID},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/tags/order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.ReorderTags(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var ordered []db.Tag
	if err := db.DB.Order("sort_order asc").Order("id asc").Find(&ordered).Error; err != nil {
		t.Fatalf("query ordered tags: %v", err)
	}
	if len(ordered) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(ordered))
	}
	if ordered[0].Name != "Gorm" || ordered[1].Name != "Go" || ordered[2].Name != "Gin" {
		t.Fatalf("unexpected order after reorder: %+v", []string{ordered[0].Name, ordered[1].Name, ordered[2].Name})
	}
	if ordered[0].SortOrder != 0 || ordered[1].SortOrder != 1 || ordered[2].SortOrder != 2 {
		t.Fatalf("unexpected sort_order after reorder: %+v", []int{ordered[0].SortOrder, ordered[1].SortOrder, ordered[2].SortOrder})
	}
}
