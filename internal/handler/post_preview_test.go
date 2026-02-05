package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
)

func TestPreviewPostRendersPostDetail(t *testing.T) {
	api, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	renderer := &stubHTMLRender{}
	router := gin.New()
	router.HTMLRender = renderer
	router.POST("/admin/posts/preview", api.PreviewPost)

	publishedAt := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	form := url.Values{}
	form.Set("content", "# 预览标题\n正文内容")
	form.Set("summary", "预览摘要")
	form.Set("tags", `[{"id":1,"name":"Go","slug":"go"}]`)
	form.Set("published_at", publishedAt.Format(time.RFC3339))

	request := httptest.NewRequest(
		http.MethodPost,
		"/admin/posts/preview",
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if renderer.lastName != "post_detail.html" {
		t.Fatalf("expected template post_detail.html, got %s", renderer.lastName)
	}

	payload, ok := renderer.lastData.(gin.H)
	if !ok {
		t.Fatalf("expected payload to be gin.H, got %T", renderer.lastData)
	}
	post, ok := payload["post"].(*db.PostPublication)
	if !ok || post == nil {
		t.Fatalf("expected payload post to be *db.PostPublication, got %T", payload["post"])
	}
	if post.Title != "预览标题" {
		t.Fatalf("expected title to be derived, got %q", post.Title)
	}
	if post.Summary != "预览摘要" {
		t.Fatalf("expected summary to match, got %q", post.Summary)
	}
	if post.ReadingTime <= 0 {
		t.Fatalf("expected reading time to be calculated, got %d", post.ReadingTime)
	}
	if len(post.Tags) != 1 || post.Tags[0].Name != "Go" {
		t.Fatalf("expected tags to be parsed, got %+v", post.Tags)
	}
}
