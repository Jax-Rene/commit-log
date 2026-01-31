package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestFormatRelativeTime(t *testing.T) {
	now := time.Date(2025, 12, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{name: "zero", input: time.Time{}, expected: ""},
		{name: "seconds", input: now.Add(-30 * time.Second), expected: "刚刚"},
		{name: "minutes", input: now.Add(-5 * time.Minute), expected: "5分钟前"},
		{name: "hours", input: now.Add(-2 * time.Hour), expected: "2小时前"},
		{name: "days", input: now.Add(-72 * time.Hour), expected: "3天前"},
		{name: "months", input: now.Add(-60 * 24 * time.Hour), expected: "2个月前"},
		{name: "years", input: now.Add(-3 * 365 * 24 * time.Hour), expected: "3年前"},
		{name: "future", input: now.Add(2 * time.Minute), expected: "刚刚"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeTime(now, tt.input)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
