package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	htmlstd "html"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	markdownEngine = goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Linkify, extension.Table),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithXHTML()),
	)
	sanitizer      = bluemonday.UGCPolicy()
	htmlTagPattern = regexp.MustCompile(`<[^>]+>`)
)

const (
	visitorCookieName   = "cl_visitor_id"
	visitorCookieMaxAge = 365 * 24 * 60 * 60
)

type tagStat struct {
	Name  string
	Count int
}

// ShowHome renders the public home page with filters and masonry layout.
func (a *API) ShowHome(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))
	tags := c.QueryArray("tags")
	page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)

	filter := service.PostFilter{
		Search:   search,
		TagNames: tags,
		Page:     page,
		PerPage:  6,
	}

	publications, err := a.posts.ListPublished(filter)
	if err != nil {
		a.renderHTML(c, http.StatusInternalServerError, "home.html", gin.H{
			"title": "首页",
			"error": "获取文章失败",
			"year":  time.Now().Year(),
		})
		return
	}

	tagOptions := a.buildTagStats()

	queryParams := buildQueryParams(search, tags)
	metaDescription := ""
	metaKeywords := make([]string, 0, len(tags)+1)
	noindex := false

	if search != "" {
		metaDescription = fmt.Sprintf("搜索“%s”的结果，共 %d 篇文章。", search, publications.Total)
		metaKeywords = append(metaKeywords, search)
		noindex = true
	}
	if len(tags) > 0 {
		tagDesc := fmt.Sprintf("当前筛选标签：%s。", strings.Join(tags, "、"))
		if metaDescription == "" {
			metaDescription = tagDesc
		} else {
			metaDescription = strings.TrimSpace(metaDescription + " " + tagDesc)
		}
		metaKeywords = append(metaKeywords, tags...)
	}
	if publications.Page > 1 && metaDescription == "" {
		metaDescription = fmt.Sprintf("第 %d 页文章列表。", publications.Page)
	}

	canonical := ""
	if search == "" && len(tags) == 0 && page == 1 {
		canonical = "/"
	}

	payload := gin.H{
		"title":       "首页",
		"search":      search,
		"tags":        tags,
		"tagOptions":  tagOptions,
		"posts":       publications.Publications,
		"page":        publications.Page,
		"totalPages":  publications.TotalPages,
		"hasMore":     publications.Page < publications.TotalPages,
		"queryParams": queryParams,
		"year":        time.Now().Year(),
	}
	if metaDescription != "" {
		payload["metaDescription"] = metaDescription
	}
	if len(metaKeywords) > 0 {
		payload["metaKeywords"] = metaKeywords
	}
	if noindex {
		payload["noindex"] = true
	}
	if canonical != "" {
		payload["canonical"] = canonical
	}

	a.renderHTML(c, http.StatusOK, "home.html", payload)
}

// LoadMorePosts returns masonry post items for infinite scroll via HTMX.
func (a *API) LoadMorePosts(c *gin.Context) {
	page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)
	if page < 2 {
		c.String(http.StatusBadRequest, "")
		return
	}

	search := strings.TrimSpace(c.Query("search"))
	tags := c.QueryArray("tags")

	filter := service.PostFilter{
		Search:   search,
		TagNames: tags,
		Page:     page,
		PerPage:  6,
	}

	publications, err := a.posts.ListPublished(filter)
	if err != nil {
		c.String(http.StatusInternalServerError, "")
		return
	}

	hasMore := page < publications.TotalPages

	a.renderHTML(c, http.StatusOK, "post_cards.html", gin.H{
		"posts":       publications.Publications,
		"hasMore":     hasMore,
		"nextPage":    page + 1,
		"queryParams": buildQueryParams(search, tags),
	})
}

// ShowPostDetail renders specific post with markdown content.
func (a *API) ShowPostDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	publication, err := a.posts.LatestPublication(uint(id))
	if err != nil {
		if errors.Is(err, service.ErrPublicationNotFound) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	postID := publication.PostID

	visitorID := a.ensureVisitorID(c)

	var (
		pageViews      uint64
		uniqueVisitors uint64
	)

	if a.analytics != nil {
		if stats, recordErr := a.analytics.RecordPostView(postID, visitorID, time.Now().UTC()); recordErr == nil {
			pageViews = stats.PageViews
			uniqueVisitors = stats.UniqueVisitors
		} else {
			c.Error(recordErr) // 不中断渲染，但记录错误
		}
	}

	contacts, contactErr := a.profiles.ListContacts(false)
	if contactErr != nil {
		contacts = nil
	}

	htmlContent, err := renderMarkdown(publication.Content)
	if err != nil {
		a.renderHTML(c, http.StatusInternalServerError, "post_detail.html", gin.H{
			"title":    "文章详情",
			"error":    "渲染内容失败",
			"year":     time.Now().Year(),
			"contacts": contacts,
		})
		return
	}
	site := a.siteSettings(c)
	description := buildPublicationDescription(publication)
	tagNames := collectTagNames(publication.Tags)
	canonicalPath := fmt.Sprintf("/posts/%d", publication.PostID)
	canonicalURL := a.absoluteURL(c, canonicalPath)

	metaImage := ""
	if cover := strings.TrimSpace(publication.CoverURL); cover != "" {
		metaImage = a.absoluteURL(c, cover)
	}

	logoURL := ""
	if site.LogoLight != "" {
		logoURL = a.absoluteURL(c, site.LogoLight)
	} else if site.LogoDark != "" {
		logoURL = a.absoluteURL(c, site.LogoDark)
	}

	jsonLD := buildPublicationJSONLD(publication, canonicalURL, site.Name, description, metaImage, logoURL, tagNames)

	publishedAt := publication.PublishedAt
	if publishedAt.IsZero() {
		publishedAt = publication.CreatedAt
	}
	modifiedAt := publication.UpdatedAt
	if modifiedAt.IsZero() {
		modifiedAt = publishedAt
	}

	payload := gin.H{
		"title":           publication.Title,
		"post":            publication,
		"content":         htmlContent,
		"contacts":        contacts,
		"pageViews":       pageViews,
		"uniqueVisitors":  uniqueVisitors,
		"year":            time.Now().Year(),
		"metaType":        "article",
		"metaPublishedAt": publishedAt,
		"metaModifiedAt":  modifiedAt,
		"canonical":       canonicalPath,
	}
	if description != "" {
		payload["metaDescription"] = description
	}
	if len(tagNames) > 0 {
		payload["metaKeywords"] = tagNames
	}
	if metaImage != "" {
		payload["metaImage"] = metaImage
	}
	if jsonLD != "" {
		payload["seoJSONLD"] = jsonLD
	}

	a.renderHTML(c, http.StatusOK, "post_detail.html", payload)
}

// ShowTagArchive lists tags and related published post counts.
func (a *API) ShowTagArchive(c *gin.Context) {
	stats := a.buildTagStats()
	tagNames := make([]string, 0, len(stats))
	for _, stat := range stats {
		if trimmed := strings.TrimSpace(stat.Name); trimmed != "" {
			tagNames = append(tagNames, trimmed)
		}
	}

	description := "当前暂无标签，快去创建一篇新文章吧。"
	if len(stats) > 0 {
		description = fmt.Sprintf("站点当前共 %d 个标签，帮助你探索不同主题。", len(stats))
	}

	payload := gin.H{
		"title":     "标签",
		"tags":      stats,
		"year":      time.Now().Year(),
		"canonical": "/tags",
		"metaType":  "website",
	}
	if description != "" {
		payload["metaDescription"] = description
	}
	if len(tagNames) > 0 {
		payload["metaKeywords"] = tagNames
	}

	a.renderHTML(c, http.StatusOK, "tag_list.html", payload)
}

func (a *API) ensureVisitorID(c *gin.Context) string {
	if id, err := c.Cookie(visitorCookieName); err == nil && strings.TrimSpace(id) != "" {
		return id
	}

	visitorID := uuid.NewString()
	secure := strings.EqualFold(a.detectScheme(c), "https")

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     visitorCookieName,
		Value:    visitorID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		MaxAge:   visitorCookieMaxAge,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	})

	return visitorID
}

// ShowAbout renders the dynamic about page.
func (a *API) ShowAbout(c *gin.Context) {
	now := time.Now().In(time.Local)
	canonical := "/about"

	contacts, contactErr := a.profiles.ListContacts(false)
	if contactErr != nil {
		contacts = nil
	}

	page, err := a.pages.GetBySlug("about")
	if err != nil {
		summary := "保持好奇心，持续输出价值。"
		a.renderHTML(c, http.StatusOK, "about.html", gin.H{
			"title": "关于",
			"page": gin.H{
				"Title":   "关于我",
				"Summary": summary,
			},
			"content":         template.HTML("<p class=\"text-sm text-slate-600\">暂无简介，稍后再来看看。</p>"),
			"year":            now.Year(),
			"contacts":        contacts,
			"metaDescription": summary,
			"metaKeywords":    []string{"关于", "个人简介"},
			"metaType":        "profile",
			"canonical":       canonical,
		})
		return
	}

	htmlContent, err := renderMarkdown(page.Content)
	if err != nil {
		htmlContent = template.HTML("<p class=\"text-sm text-slate-600\">内容暂时无法展示。</p>")
	}

	description := buildPageDescription(page)
	keywords := []string{"关于", strings.TrimSpace(page.Title)}

	payload := gin.H{
		"title":     page.Title,
		"page":      page,
		"content":   htmlContent,
		"year":      now.Year(),
		"contacts":  contacts,
		"metaType":  "profile",
		"canonical": canonical,
	}
	if description != "" {
		payload["metaDescription"] = description
	}
	payload["metaKeywords"] = keywords

	a.renderHTML(c, http.StatusOK, "about.html", payload)
}

// ShowRobots returns a dynamic robots.txt reflecting current host information.
func (a *API) ShowRobots(c *gin.Context) {
	base := strings.TrimRight(a.siteBaseURL(c), "/")
	lines := []string{
		"User-agent: *",
		"Allow: /",
		"Disallow: /admin/",
	}

	uploadPath := strings.TrimSpace(a.uploadURL)
	if uploadPath != "" {
		if !strings.HasPrefix(uploadPath, "/") {
			uploadPath = "/" + uploadPath
		}
		uploadPath = strings.TrimRight(uploadPath, "/") + "/"
		lines = append(lines, "Disallow: "+uploadPath)
	} else {
		lines = append(lines, "Disallow: /static/uploads/")
	}

	if base != "" {
		lines = append(lines, "", fmt.Sprintf("Sitemap: %s/sitemap.xml", base))
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, strings.Join(lines, "\n")+"\n")
}

// ShowSitemap exposes a simple XML sitemap for published resources.
func (a *API) ShowSitemap(c *gin.Context) {
	publications, err := a.posts.ListAllPublished()
	if err != nil {
		c.String(http.StatusInternalServerError, "")
		return
	}

	type sitemapEntry struct {
		Loc        string
		LastMod    string
		ChangeFreq string
		Priority   string
	}

	entries := []sitemapEntry{
		{Loc: a.absoluteURL(c, "/"), ChangeFreq: "daily", Priority: "1.0"},
		{Loc: a.absoluteURL(c, "/tags"), ChangeFreq: "weekly", Priority: "0.5"},
		{Loc: a.absoluteURL(c, "/about"), ChangeFreq: "yearly", Priority: "0.4"},
	}

	for _, publication := range publications {
		lastMod := publication.UpdatedAt
		if lastMod.IsZero() {
			lastMod = publication.PublishedAt
		}
		if lastMod.IsZero() {
			lastMod = publication.CreatedAt
		}
		entries = append(entries, sitemapEntry{
			Loc:        a.absoluteURL(c, fmt.Sprintf("/posts/%d", publication.PostID)),
			LastMod:    lastMod.UTC().Format(time.RFC3339),
			ChangeFreq: "weekly",
			Priority:   "0.7",
		})
	}

	var builder strings.Builder
	builder.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	builder.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, entry := range entries {
		builder.WriteString("  <url>\n")
		builder.WriteString("    <loc>" + htmlstd.EscapeString(entry.Loc) + "</loc>\n")
		if entry.LastMod != "" {
			builder.WriteString("    <lastmod>" + htmlstd.EscapeString(entry.LastMod) + "</lastmod>\n")
		}
		if entry.ChangeFreq != "" {
			builder.WriteString("    <changefreq>" + htmlstd.EscapeString(entry.ChangeFreq) + "</changefreq>\n")
		}
		if entry.Priority != "" {
			builder.WriteString("    <priority>" + htmlstd.EscapeString(entry.Priority) + "</priority>\n")
		}
		builder.WriteString("  </url>\n")
	}
	builder.WriteString("</urlset>")

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, builder.String())
}

func renderMarkdown(content string) (template.HTML, error) {
	var buf bytes.Buffer
	if err := markdownEngine.Convert([]byte(content), &buf); err != nil {
		return "", err
	}
	safe := sanitizer.SanitizeBytes(buf.Bytes())
	return template.HTML(safe), nil
}

func (a *API) buildTagStats() []tagStat {
	usages, err := a.tags.PublishedUsage()
	if err != nil {
		return nil
	}

	stats := make([]tagStat, 0, len(usages))
	for _, usage := range usages {
		if usage.Count > 0 {
			stats = append(stats, tagStat{Name: usage.Name, Count: int(usage.Count)})
		}
	}

	return stats
}

func markdownToPlainText(content string) string {
	var buf bytes.Buffer
	if err := markdownEngine.Convert([]byte(content), &buf); err != nil {
		return strings.TrimSpace(content)
	}
	safe := sanitizer.Sanitize(buf.String())
	stripped := htmlTagPattern.ReplaceAllString(safe, " ")
	unescaped := htmlstd.UnescapeString(stripped)
	return strings.Join(strings.Fields(unescaped), " ")
}

func truncateRunes(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func buildPublicationDescription(publication *db.PostPublication) string {
	if publication == nil {
		return ""
	}
	if summary := strings.TrimSpace(publication.Summary); summary != "" {
		return truncateRunes(summary, 160)
	}
	return truncateRunes(markdownToPlainText(publication.Content), 160)
}

func buildPageDescription(page *db.Page) string {
	if page == nil {
		return ""
	}
	if summary := strings.TrimSpace(page.Summary); summary != "" {
		return truncateRunes(summary, 160)
	}
	return truncateRunes(markdownToPlainText(page.Content), 160)
}

func collectTagNames(tags []db.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		if trimmed := strings.TrimSpace(tag.Name); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

func buildPublicationJSONLD(publication *db.PostPublication, canonicalURL, siteName, description, imageURL, logoURL string, tagNames []string) template.JS {
	if publication == nil {
		return ""
	}

	data := map[string]interface{}{
		"@context":         "https://schema.org",
		"@type":            "BlogPosting",
		"headline":         strings.TrimSpace(publication.Title),
		"mainEntityOfPage": map[string]interface{}{"@type": "WebPage", "@id": canonicalURL},
	}
	if canonicalURL != "" {
		data["url"] = canonicalURL
	}
	if description != "" {
		data["description"] = description
	}
	if imageURL != "" {
		data["image"] = imageURL
	}

	authorName := strings.TrimSpace(publication.User.Username)
	if authorName == "" {
		authorName = siteName
	}
	data["author"] = map[string]interface{}{
		"@type": "Person",
		"name":  authorName,
	}

	publisher := map[string]interface{}{
		"@type": "Organization",
		"name":  siteName,
	}
	if logoURL != "" {
		publisher["logo"] = map[string]interface{}{
			"@type": "ImageObject",
			"url":   logoURL,
		}
	}
	data["publisher"] = publisher

	if len(tagNames) > 0 {
		data["keywords"] = strings.Join(tagNames, ", ")
	}

	publishedAt := publication.PublishedAt
	if publishedAt.IsZero() {
		publishedAt = publication.CreatedAt
	}
	if !publishedAt.IsZero() {
		data["datePublished"] = publishedAt.UTC().Format(time.RFC3339)
	}
	updated := publication.UpdatedAt
	if updated.IsZero() {
		updated = publishedAt
	}
	if !updated.IsZero() {
		data["dateModified"] = updated.UTC().Format(time.RFC3339)
	}

	if body := markdownToPlainText(publication.Content); body != "" {
		data["articleBody"] = body
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return template.JS(encoded)
}

func buildQueryParams(search string, tags []string) string {
	values := url.Values{}
	if search != "" {
		values.Set("search", search)
	}
	for _, tag := range tags {
		if strings.TrimSpace(tag) != "" {
			values.Add("tags", strings.TrimSpace(tag))
		}
	}
	encoded := values.Encode()
	if encoded == "" {
		return ""
	}
	return "&" + encoded
}

func parsePositiveInt(value string, fallback int) int {
	num, err := strconv.Atoi(value)
	if err != nil || num <= 0 {
		return fallback
	}
	return num
}
