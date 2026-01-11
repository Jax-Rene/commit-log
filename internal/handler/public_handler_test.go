package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/router"
	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ginOnce sync.Once

func setupPublicTestDB(t *testing.T) func() {
	t.Helper()

	ginOnce.Do(func() {
		gin.SetMode(gin.TestMode)
	})

	dsn := fmt.Sprintf("file:public-handler-%d?mode=memory&cache=shared", time.Now().UnixNano())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := gdb.AutoMigrate(&db.User{}, &db.Post{}, &db.PostPublication{}, &db.Tag{}, &db.TagTranslation{}, &db.Page{}, &db.ProfileContact{}, &db.PostStatistic{}, &db.PostVisit{}, &db.SystemSetting{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	if err := gdb.Create(&db.User{Username: "tester", Password: "hashed"}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	db.DB = gdb

	return func() {
		sqlDB, err := db.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func seedPublishedPostAt(t *testing.T, title, content string, publishedAt time.Time) db.Post {
	return seedPublishedPostAtWithLanguage(t, title, content, publishedAt, "zh", 0)
}

func seedPublishedPostAtWithLanguage(t *testing.T, title, content string, publishedAt time.Time, language string, groupID uint) db.Post {
	t.Helper()

	summary := fmt.Sprintf("%s 摘要", title)
	post := db.Post{
		Title:              title,
		Content:            content,
		Summary:            summary,
		Status:             "draft",
		UserID:             1,
		CoverURL:           fmt.Sprintf("https://images.unsplash.com/photo-1500530855697-b586d89ba3ee?title=%s", urlSafe(title)),
		CoverWidth:         1280,
		CoverHeight:        720,
		Language:           language,
		TranslationGroupID: groupID,
	}
	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to create post: %v", err)
	}
	if post.TranslationGroupID == 0 {
		if err := db.DB.Model(&post).Update("translation_group_id", post.ID).Error; err != nil {
			t.Fatalf("failed to set translation group id: %v", err)
		}
		post.TranslationGroupID = post.ID
	}

	publication := db.PostPublication{
		PostID:      post.ID,
		Title:       post.Title,
		Content:     post.Content,
		Summary:     post.Summary,
		ReadingTime: 1,
		CoverURL:    post.CoverURL,
		CoverWidth:  post.CoverWidth,
		CoverHeight: post.CoverHeight,
		UserID:      post.UserID,
		PublishedAt: publishedAt,
		Version:     1,
	}
	if err := db.DB.Create(&publication).Error; err != nil {
		t.Fatalf("failed to create publication: %v", err)
	}

	updates := map[string]any{
		"status":                "published",
		"published_at":          publication.PublishedAt,
		"publication_count":     1,
		"latest_publication_id": publication.ID,
	}
	if err := db.DB.Model(&post).Updates(updates).Error; err != nil {
		t.Fatalf("failed to update post metadata: %v", err)
	}

	if err := db.DB.First(&post, post.ID).Error; err != nil {
		t.Fatalf("failed to reload post: %v", err)
	}

	return post
}

func seedPublishedPost(t *testing.T, title, content string) db.Post {
	return seedPublishedPostAt(t, title, content, time.Now())
}

func seedPublishedPostWithLanguage(t *testing.T, title, content, language string) db.Post {
	return seedPublishedPostAtWithLanguage(t, title, content, time.Now(), language, 0)
}

func seedPublishedPostWithLanguageGroup(t *testing.T, title, content, language string, groupID uint) db.Post {
	return seedPublishedPostAtWithLanguage(t, title, content, time.Now(), language, groupID)
}

func seedDraftPost(t *testing.T, title string) db.Post {
	t.Helper()
	post := db.Post{
		Title:       title,
		Content:     "草稿内容",
		Status:      "draft",
		UserID:      1,
		CoverURL:    fmt.Sprintf("https://images.unsplash.com/photo-1441986300917-64674bd600d8?title=%s", urlSafe(title)),
		CoverWidth:  960,
		CoverHeight: 1280,
		Language:    "zh",
	}
	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("failed to create draft: %v", err)
	}
	if err := db.DB.Model(&post).Update("translation_group_id", post.ID).Error; err != nil {
		t.Fatalf("failed to set translation group id: %v", err)
	}
	return post
}

func urlSafe(input string) string {
	return strings.ReplaceAll(input, " ", "+")
}

func TestShowHomeExcludesDrafts(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	published := seedPublishedPost(t, "Published Post", "内容")
	draft := seedDraftPost(t, "Draft Post")

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, published.Title) {
		t.Fatalf("expected response to include published post title")
	}
	if strings.Contains(body, draft.Title) {
		t.Fatalf("draft post should not be rendered on public home")
	}
}

func TestShowHomeUsesCountryLanguage(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	zhPost := seedPublishedPostWithLanguage(t, "中文文章", "内容", "zh")
	enPost := seedPublishedPostWithLanguage(t, "English Post", "Content", "en")

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")

	cnResp := httptest.NewRecorder()
	cnReq := httptest.NewRequest(http.MethodGet, "/", nil)
	cnReq.Header.Set("CF-IPCountry", "CN")
	r.ServeHTTP(cnResp, cnReq)

	if cnResp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", cnResp.Code)
	}
	if !strings.Contains(cnResp.Body.String(), zhPost.Summary) {
		t.Fatalf("expected cn response to include zh content")
	}
	if strings.Contains(cnResp.Body.String(), enPost.Summary) {
		t.Fatalf("expected cn response to exclude en content")
	}

	enResp := httptest.NewRecorder()
	enReq := httptest.NewRequest(http.MethodGet, "/", nil)
	enReq.Header.Set("CF-IPCountry", "US")
	r.ServeHTTP(enResp, enReq)

	if enResp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", enResp.Code)
	}
	if !strings.Contains(enResp.Body.String(), enPost.Summary) {
		t.Fatalf("expected en response to include en content")
	}
	if strings.Contains(enResp.Body.String(), zhPost.Summary) {
		t.Fatalf("expected en response to exclude zh content")
	}
}

func TestShowHomeLocalizesPostCardTagNames(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	post := seedPublishedPostWithLanguage(t, "English Post", "Content", "en")

	tag := db.Tag{Name: "产品"}
	if err := db.DB.Create(&tag).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}
	if err := db.DB.Create(&db.TagTranslation{TagID: tag.ID, Language: "zh", Name: "产品"}).Error; err != nil {
		t.Fatalf("failed to seed zh tag translation: %v", err)
	}
	if err := db.DB.Create(&db.TagTranslation{TagID: tag.ID, Language: "en", Name: "Product"}).Error; err != nil {
		t.Fatalf("failed to seed en tag translation: %v", err)
	}

	if err := db.DB.Model(&post).Association("Tags").Append(&tag); err != nil {
		t.Fatalf("failed to associate post tag: %v", err)
	}
	if post.LatestPublicationID == nil || *post.LatestPublicationID == 0 {
		t.Fatalf("expected post to have latest publication id")
	}
	var publication db.PostPublication
	if err := db.DB.First(&publication, *post.LatestPublicationID).Error; err != nil {
		t.Fatalf("failed to load publication: %v", err)
	}
	if err := db.DB.Model(&publication).Association("Tags").Append(&tag); err != nil {
		t.Fatalf("failed to associate publication tag: %v", err)
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?lang=en", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Product") {
		t.Fatalf("expected response to include localized tag name")
	}
	if strings.Contains(body, "产品") {
		t.Fatalf("expected response to not include tag key name when lang=en")
	}
}

func TestShowHomeUsesPreferredLanguageFallback(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	systemService := service.NewSystemSettingService(db.DB)
	if _, err := systemService.UpdateSettings(service.SystemSettingsInput{
		PreferredLanguage: "en",
	}); err != nil {
		t.Fatalf("failed to update preferred language: %v", err)
	}

	zhPost := seedPublishedPostWithLanguage(t, "中文文章", "内容", "zh")
	enPost := seedPublishedPostWithLanguage(t, "English Post", "Content", "en")

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("CF-IPCountry", "CN")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), enPost.Summary) {
		t.Fatalf("expected preferred language to include en content")
	}
	if strings.Contains(w.Body.String(), zhPost.Summary) {
		t.Fatalf("expected preferred language to exclude zh content")
	}
}

func TestShowHomePreferredLanguageSetsCookie(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	systemService := service.NewSystemSettingService(db.DB)
	if _, err := systemService.UpdateSettings(service.SystemSettingsInput{
		PreferredLanguage: "en",
	}); err != nil {
		t.Fatalf("failed to update preferred language: %v", err)
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	result := w.Result()
	defer result.Body.Close()

	found := false
	for _, cookie := range result.Cookies() {
		if cookie.Name == "cl_lang" {
			found = true
			if cookie.Value != "en" {
				t.Fatalf("expected cl_lang cookie to be en, got %q", cookie.Value)
			}
		}
	}
	if !found {
		t.Fatalf("expected cl_lang cookie to be set")
	}
}

func TestShowHomeHonorsLangQueryParam(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	zhPost := seedPublishedPostWithLanguage(t, "中文文章", "内容", "zh")
	enPost := seedPublishedPostWithLanguage(t, "English Post", "Content", "en")

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?lang=en", nil)
	req.Header.Set("CF-IPCountry", "CN")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), enPost.Summary) {
		t.Fatalf("expected lang query to include en content")
	}
	if strings.Contains(w.Body.String(), zhPost.Summary) {
		t.Fatalf("expected lang query to exclude zh content")
	}
}

func TestShowHomeHonorsLangCookie(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	zhPost := seedPublishedPostWithLanguage(t, "中文文章", "内容", "zh")
	enPost := seedPublishedPostWithLanguage(t, "English Post", "Content", "en")

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("CF-IPCountry", "CN")
	req.AddCookie(&http.Cookie{Name: "cl_lang", Value: "en"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), enPost.Summary) {
		t.Fatalf("expected lang cookie to include en content")
	}
	if strings.Contains(w.Body.String(), zhPost.Summary) {
		t.Fatalf("expected lang cookie to exclude zh content")
	}
}

func TestShowPostDetailRedirectsToTranslation(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	zhPost := seedPublishedPostWithLanguage(t, "中文文章", "# 中文文章\n正文", "zh")
	enPost := seedPublishedPostWithLanguageGroup(t, "English Post", "# English Post\nBody", "en", zhPost.TranslationGroupID)

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+strconv.Itoa(int(zhPost.ID)), nil)
	req.Header.Set("CF-IPCountry", "US")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	expected := fmt.Sprintf("/posts/%d", enPost.ID)
	if location != expected {
		t.Fatalf("expected redirect to %s, got %s", expected, location)
	}
}

func TestHomeCanonicalUsesConfiguredBaseURL(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "https://blog.jaxrene.dev")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	expected := `rel="canonical" href="https://blog.jaxrene.dev/"`
	if !strings.Contains(w.Body.String(), expected) {
		t.Fatalf("expected canonical link %s", expected)
	}
}

func TestLoadMorePostsHandlesPagination(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	now := time.Now()
	for i := 1; i <= 7; i++ {
		title := "Post " + strconv.Itoa(i)
		seedPublishedPostAt(t, title, "内容", now.Add(-time.Duration(i)*time.Minute))
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/more?page=2", nil)
	req.Header.Set("HX-Request", "true")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Post 7") {
		t.Fatalf("expected paginated response to include oldest post")
	}
	if strings.Contains(body, "Post 1") {
		t.Fatalf("expected second page to exclude first page items")
	}
}

func TestShowPostDetailRejectsDraft(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	draft := seedDraftPost(t, "Drafted")

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+strconv.Itoa(int(draft.ID)), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for draft post, got %d", w.Code)
	}
}

func TestShowAboutFallback(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "About Me") {
		t.Fatalf("expected fallback about title in response")
	}
}

func TestShowAboutDisplaysContacts(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	aboutPage := db.Page{Slug: "about", Title: "About Me", Content: "# 你好", Language: "zh"}
	if err := db.DB.Create(&aboutPage).Error; err != nil {
		t.Fatalf("failed to seed about page: %v", err)
	}

	contacts := []db.ProfileContact{
		{Platform: "微信", Label: "个人微信", Value: "coder-123", Icon: "wechat", Sort: 0, Visible: true},
		{Platform: "GitHub", Label: "GitHub", Value: "https://github.com/commitlog", Link: "https://github.com/commitlog", Icon: "github", Sort: 1, Visible: true},
	}
	if err := db.DB.Create(&contacts).Error; err != nil {
		t.Fatalf("failed to seed contacts: %v", err)
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "个人微信") {
		t.Fatalf("expected contact label to render")
	}
	if !strings.Contains(body, "https://github.com/commitlog") {
		t.Fatalf("expected contact link to render")
	}
	if !strings.Contains(body, "联系我") {
		t.Fatalf("expected contact section heading")
	}
}

func TestShowAboutHidesSummary(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	aboutPage := db.Page{Slug: "about", Title: "About Me", Content: "# 你好", Summary: "不应显示摘要", Language: "zh"}
	if err := db.DB.Create(&aboutPage).Error; err != nil {
		t.Fatalf("failed to seed about page: %v", err)
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	bodyStart := strings.Index(body, "<body")
	if bodyStart == -1 {
		t.Fatalf("expected body tag in response")
	}
	if strings.Contains(body[bodyStart:], "不应显示摘要") {
		t.Fatalf("expected about page to hide summary in visible content")
	}
}

func TestShowAboutUsesSummaryForMeta(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	aboutPage := db.Page{Slug: "about", Title: "About Me", Content: "# 正文内容", Summary: "自定义关于页摘要", Language: "zh"}
	if err := db.DB.Create(&aboutPage).Error; err != nil {
		t.Fatalf("failed to seed about page: %v", err)
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `name="description" content="自定义关于页摘要"`) {
		t.Fatalf("expected meta description to prefer page summary, body=%s", body)
	}
}

func TestShowAboutRespectsLanguage(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	zhPage := db.Page{Slug: "about", Title: "关于我", Content: "# 你好", Language: "zh"}
	if err := db.DB.Create(&zhPage).Error; err != nil {
		t.Fatalf("failed to seed zh about page: %v", err)
	}
	enPage := db.Page{Slug: "about", Title: "About Me", Content: "# Hello", Language: "en"}
	if err := db.DB.Create(&enPage).Error; err != nil {
		t.Fatalf("failed to seed en about page: %v", err)
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")

	cnResp := httptest.NewRecorder()
	cnReq := httptest.NewRequest(http.MethodGet, "/about", nil)
	cnReq.Header.Set("CF-IPCountry", "CN")
	r.ServeHTTP(cnResp, cnReq)

	if cnResp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", cnResp.Code)
	}
	if !strings.Contains(cnResp.Body.String(), "关于我") {
		t.Fatalf("expected zh about content")
	}

	enResp := httptest.NewRecorder()
	enReq := httptest.NewRequest(http.MethodGet, "/about", nil)
	enReq.Header.Set("CF-IPCountry", "US")
	r.ServeHTTP(enResp, enReq)

	if enResp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", enResp.Code)
	}
	if !strings.Contains(enResp.Body.String(), "About Me") {
		t.Fatalf("expected en about content")
	}
}

func TestShowPostDetailDisplaysContacts(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	post := seedPublishedPost(t, "公开文章", "## 内容")

	contact := db.ProfileContact{Platform: "邮箱", Label: "联系邮箱", Value: "hi@example.com", Link: "mailto:hi@example.com", Icon: "email", Sort: 0, Visible: true}
	if err := db.DB.Create(&contact).Error; err != nil {
		t.Fatalf("failed to seed contact: %v", err)
	}

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+strconv.Itoa(int(post.ID)), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "联系作者") {
		t.Fatalf("expected contact banner heading")
	}
	if !strings.Contains(body, "mailto:hi@example.com") {
		t.Fatalf("expected contact link to render")
	}
}

func TestShowPostDetailStripsLeadingTitleFromContent(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	content := "# 公开文章\n\n正文段落"
	post := seedPublishedPost(t, "公开文章", content)

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+strconv.Itoa(int(post.ID)), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, "# 公开文章") {
		t.Fatalf("expected rendered content to exclude leading markdown title")
	}
	if !strings.Contains(body, "正文段落") {
		t.Fatalf("expected rendered content to retain body text")
	}
}

func TestRSSFeedIncludesPublishedPosts(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	published := seedPublishedPostAt(t, "RSS 测试", "# RSS 测试\n\n正文", time.Date(2024, 11, 23, 10, 0, 0, 0, time.UTC))

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rss.xml", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/rss+xml") {
		t.Fatalf("expected RSS content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<rss version=\"2.0\"") {
		t.Fatalf("expected RSS root element, body=%s", body)
	}
	if !strings.Contains(body, "<title>RSS 测试</title>") {
		t.Fatalf("expected feed to include post title, body=%s", body)
	}
	if !strings.Contains(body, fmt.Sprintf("/posts/%d", published.ID)) {
		t.Fatalf("expected feed to include post URL")
	}
	if !strings.Contains(body, fmt.Sprintf("<description>%s 摘要</description>", "RSS 测试")) {
		t.Fatalf("expected feed to include summary description, body=%s", body)
	}
	if !strings.Contains(body, "<content:encoded><![CDATA[") {
		t.Fatalf("expected feed to include full content payload, body=%s", body)
	}
	if strings.Contains(body, "<h1>RSS 测试</h1>") {
		t.Fatalf("expected feed content to exclude leading title heading, body=%s", body)
	}
	if !strings.Contains(body, fmt.Sprintf("<img src=\"%s\"", published.CoverURL)) {
		t.Fatalf("expected feed content to include cover image, body=%s", body)
	}
	if !strings.Contains(body, "<p>正文</p>") {
		t.Fatalf("expected feed content to include rendered HTML body, body=%s", body)
	}
	if !strings.Contains(body, fmt.Sprintf("<media:content url=\"%s\"", published.CoverURL)) {
		t.Fatalf("expected feed to include cover media content, body=%s", body)
	}
}

func TestHomeDisplaysRSSLink(t *testing.T) {
	cleanup := setupPublicTestDB(t)
	defer cleanup()

	r := router.SetupRouter("test-secret", "web/static/uploads", "/static/uploads", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `href="/rss.xml"`) {
		t.Fatalf("expected home page to include RSS link, body=%s", body)
	}
	if !strings.Contains(body, ">RSS<") {
		t.Fatalf("expected RSS label to render, body=%s", body)
	}
}
