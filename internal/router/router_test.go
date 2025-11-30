package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
)

func TestSetupRouterServesUploadsAlias(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db.DB = nil

	uploadDir := t.TempDir()
	fileName := "example.txt"
	fileContent := []byte("hello uploads")
	if err := os.WriteFile(filepath.Join(uploadDir, fileName), fileContent, 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	r := SetupRouter("test-secret", uploadDir, "/static/uploads", "")

	req := httptest.NewRequest(http.MethodGet, "/uploads/"+fileName, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Body.String() != string(fileContent) {
		t.Fatalf("unexpected body, got %q", rr.Body.String())
	}
}
