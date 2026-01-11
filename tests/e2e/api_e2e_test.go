package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/router"
	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type e2eSuite struct {
	handler      http.Handler
	public       httpClient
	admin        httpClient
	baseURL      string
	uploadDir    string
	adminPass    string
	user         db.User
	tags         []db.Tag
	published    *db.Post
	draft        *db.Post
	contacts     []db.ProfileContact
	aboutContent string
}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type localClient struct {
	handler http.Handler
	jar     http.CookieJar
}

func newLocalClient(handler http.Handler, withJar bool) *localClient {
	var jar http.CookieJar
	if withJar {
		if j, err := cookiejar.New(nil); err == nil {
			jar = j
		}
	}
	return &localClient{handler: handler, jar: jar}
}

func (c *localClient) Do(req *http.Request) (*http.Response, error) {
	if c.jar != nil {
		for _, cookie := range c.jar.Cookies(req.URL) {
			req.AddCookie(cookie)
		}
	}
	w := httptest.NewRecorder()
	c.handler.ServeHTTP(w, req)
	resp := w.Result()
	if c.jar != nil {
		c.jar.SetCookies(req.URL, resp.Cookies())
	}
	return resp, nil
}

func TestE2E_AllInterfaces(t *testing.T) {
	suite := newE2ESuite(t)
	suite.login(t)

	t.Run("public endpoints", suite.testPublicEndpoints)
	t.Run("admin pages", suite.testAdminPages)
	suite.login(t) // 确保后续 API 测试有有效会话
	t.Run("admin apis", suite.testAdminAPIs)
}

func newE2ESuite(t *testing.T) *e2eSuite {
	t.Helper()
	gin.SetMode(gin.TestMode)

	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := gdb.AutoMigrate(
		&db.User{},
		&db.Post{},
		&db.PostPublication{},
		&db.Tag{},
		&db.TagTranslation{},
		&db.Page{},
		&db.ProfileContact{},
		&db.PostStatistic{},
		&db.PostVisit{},
		&db.SiteHourlySnapshot{},
		&db.SiteHourlyVisitor{},
		&db.SystemSetting{},
	); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	db.DB = gdb

	hashed, err := bcrypt.GenerateFromPassword([]byte("e2e-secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := db.User{Username: "admin", Password: string(hashed)}
	if err := db.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	tags := []db.Tag{{Name: "Go"}, {Name: "AI"}}
	if err := db.DB.Create(&tags).Error; err != nil {
		t.Fatalf("failed to seed tags: %v", err)
	}

	postSvc := service.NewPostService(db.DB)
	published, err := postSvc.Create(service.PostInput{
		Title:       "已发布文章",
		Content:     "# 已发布文章\n这是正文内容。",
		Summary:     "已发布摘要",
		TagIDs:      []uint{tags[0].ID},
		UserID:      user.ID,
		CoverURL:    "https://example.com/cover.jpg",
		CoverWidth:  1200,
		CoverHeight: 800,
	})
	if err != nil {
		t.Fatalf("failed to seed published post: %v", err)
	}
	if _, err := postSvc.Publish(published.ID, user.ID, ptrTime(time.Now().UTC())); err != nil {
		t.Fatalf("failed to publish seeded post: %v", err)
	}

	draft, err := postSvc.Create(service.PostInput{
		Title:       "草稿文章",
		Content:     "# 草稿文章\n待发布的内容。",
		Summary:     "草稿摘要",
		TagIDs:      []uint{tags[1].ID},
		UserID:      user.ID,
		CoverURL:    "https://example.com/draft.jpg",
		CoverWidth:  800,
		CoverHeight: 600,
	})
	if err != nil {
		t.Fatalf("failed to seed draft post: %v", err)
	}

	pageSvc := service.NewPageService(db.DB)
	aboutContent := "## About Me\n这是 E2E 关于页面的测试内容。"
	if _, err := pageSvc.SaveAboutPage(aboutContent, "zh"); err != nil {
		t.Fatalf("failed to seed about page: %v", err)
	}

	profileSvc := service.NewProfileService(db.DB)
	contactA, err := profileSvc.CreateContact(service.ProfileContactInput{
		Platform: "github",
		Label:    "GitHub",
		Value:    "commitlog",
		Link:     "",
	})
	if err != nil {
		t.Fatalf("failed to seed profile contact: %v", err)
	}
	contactB, err := profileSvc.CreateContact(service.ProfileContactInput{
		Platform: "wechat",
		Label:    "WeChat",
		Value:    "wechat-id",
		Link:     "https://example.com/wechat.png",
	})
	if err != nil {
		t.Fatalf("failed to seed wechat contact: %v", err)
	}

	uploadDir := t.TempDir()
	engine := router.SetupRouter("test-session-secret", uploadDir, "/uploads", "http://example.test")

	return &e2eSuite{
		handler:      engine,
		public:       newLocalClient(engine, false),
		admin:        newLocalClient(engine, true),
		baseURL:      "http://example.test",
		uploadDir:    uploadDir,
		adminPass:    "e2e-secret",
		user:         user,
		tags:         tags,
		published:    published,
		draft:        draft,
		contacts:     []db.ProfileContact{*contactA, *contactB},
		aboutContent: aboutContent,
	}
}

func (s *e2eSuite) login(t *testing.T) {
	t.Helper()
	form := url.Values{
		"username": {s.user.Username},
		"password": {s.adminPass},
		"remember": {"1"},
	}

	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/admin/login", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("failed to create login request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.admin.Do(req)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed, status %d", resp.StatusCode)
	}
}

func (s *e2eSuite) testPublicEndpoints(t *testing.T) {
	t.Helper()
	publishedID := s.published.ID

	checkHTML := func(name, path, expect string, code int) {
		t.Helper()
		resp := s.mustRequest(t, s.public, http.MethodGet, path, nil, nil)
		defer resp.Body.Close()
		if resp.StatusCode != code {
			t.Fatalf("%s: expected status %d, got %d", name, code, resp.StatusCode)
		}
		body := readBody(t, resp)
		if expect != "" && !strings.Contains(body, expect) {
			t.Fatalf("%s: response does not contain %q", name, expect)
		}
	}

	checkHTML("home", "/", "已发布文章", http.StatusOK)
	checkHTML("post detail", "/posts/"+idStr(publishedID), "已发布文章", http.StatusOK)
	checkHTML("tags page", "/tags", "标签", http.StatusOK)
	checkHTML("about page", "/about", "About Me", http.StatusOK)
	checkHTML("load more", "/posts/more?page=2", "", http.StatusOK)
	checkHTML("search suggestions", "/search/suggestions?search=已发布", "posts/"+idStr(publishedID), http.StatusOK)
	checkHTML("robots", "/robots.txt", "User-agent", http.StatusOK)
	checkHTML("sitemap", "/sitemap.xml", "<urlset", http.StatusOK)
	checkHTML("rss", "/rss.xml", "<rss", http.StatusOK)

	resp := s.mustRequest(t, s.public, http.MethodGet, "/ping", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ping: expected 200, got %d", resp.StatusCode)
	}
	if body := readBody(t, resp); !strings.Contains(body, "pong") {
		t.Fatalf("ping: unexpected body %q", body)
	}

	resp = s.mustRequest(t, s.public, http.MethodGet, "/healthz", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: expected 200, got %d", resp.StatusCode)
	}
	if body := readBody(t, resp); !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("healthz: unexpected body %q", body)
	}
}

func (s *e2eSuite) testAdminPages(t *testing.T) {
	t.Helper()
	needs200 := []string{
		"/admin/dashboard",
		"/admin/posts",
		"/admin/posts/new",
		"/admin/posts/" + idStr(s.published.ID) + "/edit",
		"/admin/tags",
		"/admin/about",
		"/admin/system/settings",
	}

	for _, path := range needs200 {
		resp := s.mustRequest(t, s.admin, http.MethodGet, path, nil, nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s expected 200, got %d", path, resp.StatusCode)
		}
	}

	resp := s.mustRequest(t, s.admin, http.MethodGet, "/admin/profile/contacts", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("/admin/profile/contacts expected 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/admin/system/settings") {
		t.Fatalf("unexpected redirect location %q", loc)
	}

	resp = s.mustRequest(t, s.admin, http.MethodGet, "/admin/logout", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("logout expected 302, got %d", resp.StatusCode)
	}
}

func (s *e2eSuite) testAdminAPIs(t *testing.T) {
	t.Helper()

	resp := s.mustRequest(t, s.admin, http.MethodGet, "/admin/api/posts", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list posts expected 200, got %d", resp.StatusCode)
	}
	var listPayload map[string]interface{}
	decodeJSON(t, resp, &listPayload)

	resp = s.mustRequest(t, s.admin, http.MethodGet, "/admin/api/posts/"+idStr(s.published.ID), nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get post expected 200, got %d", resp.StatusCode)
	}

	newPostPayload := map[string]interface{}{
		"title":        "E2E 新文章",
		"content":      "# E2E 新文章\n测试内容。",
		"summary":      "E2E 摘要",
		"tag_ids":      []uint{s.tags[0].ID},
		"cover_url":    "https://example.com/new.jpg",
		"cover_width":  640,
		"cover_height": 480,
	}
	resp = s.mustRequestJSON(t, s.admin, http.MethodPost, "/admin/api/posts", newPostPayload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create post expected 200, got %d", resp.StatusCode)
	}
	var created struct {
		Post struct {
			ID uint `json:"id"`
		} `json:"post"`
	}
	decodeJSON(t, resp, &created)
	if created.Post.ID == 0 {
		t.Fatalf("create post returned empty id")
	}

	updatePayload := map[string]interface{}{
		"title":        "E2E 新文章",
		"content":      "# E2E 新文章\n更新后的内容。",
		"summary":      "更新后的摘要",
		"tag_ids":      []uint{s.tags[0].ID},
		"cover_url":    "https://example.com/new.jpg",
		"cover_width":  640,
		"cover_height": 480,
	}
	updatePath := "/admin/api/posts/" + idStr(created.Post.ID)
	resp = s.mustRequestJSON(t, s.admin, http.MethodPut, updatePath, updatePayload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update post expected 200, got %d", resp.StatusCode)
	}

	publishPath := "/admin/api/posts/" + idStr(created.Post.ID) + "/publish"
	resp = s.mustRequest(t, s.admin, http.MethodPost, publishPath, strings.NewReader("published_at="), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("publish post expected 200, got %d, body=%s", resp.StatusCode, readBody(t, resp))
	}

	resp = s.mustRequest(t, s.admin, http.MethodDelete, updatePath, nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete post expected 200, got %d", resp.StatusCode)
	}

	s.assertAIEndpointFails(t, "/admin/api/posts/summary", map[string]interface{}{
		"title":   "AI 摘要",
		"content": "需要摘要的内容",
	})
	s.assertAIEndpointFails(t, "/admin/api/posts/optimize", map[string]interface{}{
		"title":   "AI 优化",
		"content": "需要优化的内容",
	})
	s.assertAIEndpointFails(t, "/admin/api/posts/chat", map[string]interface{}{
		"selection":   "需要改写的段落",
		"instruction": "请改写语气",
		"context":     "上下文",
	})

	resp = s.mustRequest(t, s.admin, http.MethodGet, "/admin/api/tags", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list tags expected 200, got %d", resp.StatusCode)
	}

	resp = s.mustRequestJSON(t, s.admin, http.MethodPost, "/admin/api/tags", map[string]interface{}{"name": "e2e-tag"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create tag expected 200, got %d", resp.StatusCode)
	}
	var tagCreated struct {
		Tag db.Tag `json:"tag"`
	}
	decodeJSON(t, resp, &tagCreated)
	tagID := tagCreated.Tag.ID

	resp = s.mustRequestJSON(t, s.admin, http.MethodPut, "/admin/api/tags/"+idStr(tagID), map[string]interface{}{"name": "e2e-tag-updated"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update tag expected 200, got %d", resp.StatusCode)
	}

	resp = s.mustRequest(t, s.admin, http.MethodDelete, "/admin/api/tags/"+idStr(tagID), nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete tag expected 200, got %d", resp.StatusCode)
	}

	resp = s.mustRequestJSON(t, s.admin, http.MethodPut, "/admin/api/pages/about", map[string]interface{}{"content": "更新后的关于页面"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update about expected 200, got %d", resp.StatusCode)
	}

	resp = s.mustRequest(t, s.admin, http.MethodGet, "/admin/api/profile/contacts", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list contacts expected 200, got %d", resp.StatusCode)
	}

	resp = s.mustRequestJSON(t, s.admin, http.MethodPost, "/admin/api/profile/contacts", map[string]interface{}{
		"platform": "telegram",
		"label":    "TG",
		"value":    "jaxbot",
		"link":     "",
		"icon":     "telegram",
		"sort":     5,
		"visible":  true,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create contact expected 201, got %d", resp.StatusCode)
	}
	var contactCreated struct {
		Contact db.ProfileContact `json:"contact"`
	}
	decodeJSON(t, resp, &contactCreated)
	newContactID := contactCreated.Contact.ID

	resp = s.mustRequestJSON(t, s.admin, http.MethodPut, "/admin/api/profile/contacts/"+idStr(newContactID), map[string]interface{}{
		"platform": "telegram",
		"label":    "TG 更新",
		"value":    "jaxbot",
		"link":     "",
		"icon":     "telegram",
		"sort":     6,
		"visible":  true,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update contact expected 200, got %d", resp.StatusCode)
	}

	orderPayload := map[string]interface{}{
		"ids": []uint{newContactID, s.contacts[0].ID, s.contacts[1].ID},
	}
	resp = s.mustRequestJSON(t, s.admin, http.MethodPut, "/admin/api/profile/contacts/order", orderPayload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reorder contacts expected 200, got %d", resp.StatusCode)
	}

	resp = s.mustRequest(t, s.admin, http.MethodDelete, "/admin/api/profile/contacts/"+idStr(newContactID), nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete contact expected 200, got %d", resp.StatusCode)
	}

	resp = s.mustRequest(t, s.admin, http.MethodGet, "/admin/api/system/settings", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get system settings expected 200, got %d", resp.StatusCode)
	}

	settingsPayload := map[string]interface{}{
		"siteName":         "E2E 站点",
		"siteLogoUrl":      "https://example.com/logo.png",
		"siteLogoUrlLight": "https://example.com/logo-light.png",
		"siteLogoUrlDark":  "https://example.com/logo-dark.png",
		"siteDescription":  "端到端测试站点",
		"siteSocialImage":  "https://example.com/social.png",
		"aiProvider":       "openai",
		"openaiApiKey":     "",
		"deepseekApiKey":   "",
		"adminFooterText":  "footer admin",
		"publicFooterText": "footer public",
		"aiSummaryPrompt":  "summary prompt",
		"aiRewritePrompt":  "rewrite prompt",
	}
	resp = s.mustRequestJSON(t, s.admin, http.MethodPut, "/admin/api/system/settings", settingsPayload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update system settings expected 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "E2E 站点") {
		t.Fatalf("system settings response missing site name: %s", body)
	}

	resp = s.mustRequestJSON(t, s.admin, http.MethodPost, "/admin/api/system/settings/ai/test", map[string]interface{}{
		"provider": "openai",
		"apiKey":   "",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ai test expected 400 when api key missing, got %d", resp.StatusCode)
	}

	resp = s.uploadTestImage(t)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload image expected 200, got %d, body=%s", resp.StatusCode, readBody(t, resp))
	}
	var uploadResp struct {
		Success int `json:"success"`
		Data    struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	decodeJSON(t, resp, &uploadResp)
	if uploadResp.Success != 1 || uploadResp.Data.URL == "" {
		t.Fatalf("unexpected upload response: %+v", uploadResp)
	}
}

func (s *e2eSuite) assertAIEndpointFails(t *testing.T, path string, payload map[string]interface{}) {
	t.Helper()
	resp := s.mustRequestJSON(t, s.admin, http.MethodPost, path, payload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("%s expected 400 without API key, got %d", path, resp.StatusCode)
	}
}

func (s *e2eSuite) uploadTestImage(t *testing.T) *http.Response {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 200, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "image", "test.png"))
	partHeader.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(buf.Bytes()); err != nil {
		t.Fatalf("failed to write image: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	headers := map[string]string{
		"Content-Type": writer.FormDataContentType(),
	}
	return s.mustRequest(t, s.admin, http.MethodPost, "/admin/api/upload/image", body, headers)
}

func (s *e2eSuite) mustRequest(t *testing.T, client httpClient, method, path string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, s.baseURL+path, body)
	if err != nil {
		t.Fatalf("failed to build request %s %s: %v", method, path, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, path, err)
	}
	return resp
}

func (s *e2eSuite) mustRequestJSON(t *testing.T, client httpClient, method, path string, payload map[string]interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	return s.mustRequest(t, client, method, path, bytes.NewReader(data), headers)
}

func decodeJSON(t *testing.T, resp *http.Response, dst interface{}) {
	t.Helper()
	body := readBody(t, resp)
	if err := json.Unmarshal([]byte(body), dst); err != nil {
		t.Fatalf("failed to decode json: %v\nbody=%s", err, body)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return string(data)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func idStr(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
