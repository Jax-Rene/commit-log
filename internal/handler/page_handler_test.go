package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
)

func TestUpdateAboutPageCreatesRecord(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	payload := map[string]string{"content": "# About Me\n这是新的介绍"}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/admin/api/pages/about", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	api.UpdateAboutPage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var count int64
	db.DB.Model(&db.Page{}).Where("slug = ? AND language = ?", "about", "zh").Count(&count)
	if count != 1 {
		t.Fatalf("expected about page to be created, found %d", count)
	}
}
