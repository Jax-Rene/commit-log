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
	cleanup := setupTestDB(t)
	defer cleanup()

	payload := map[string]string{"content": "# 关于我\n这是新的介绍"}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/admin/api/pages/about", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateAboutPage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var count int64
	db.DB.Model(&db.Page{}).Where("slug = ?", "about").Count(&count)
	if count != 1 {
		t.Fatalf("expected about page to be created, found %d", count)
	}
}
