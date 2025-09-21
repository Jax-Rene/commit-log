package handler

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
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
	sanitizer = bluemonday.UGCPolicy()
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
		Status:   "published",
		TagNames: tags,
		Page:     page,
		PerPage:  6,
	}

	posts, err := a.posts.List(filter)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "home.html", gin.H{
			"title": "首页",
			"error": "获取文章失败",
			"year":  time.Now().Year(),
		})
		return
	}

	tagOptions := a.buildTagStats()

	queryParams := buildQueryParams(search, tags)

	c.HTML(http.StatusOK, "home.html", gin.H{
		"title":       "首页",
		"search":      search,
		"tags":        tags,
		"tagOptions":  tagOptions,
		"posts":       posts.Posts,
		"page":        posts.Page,
		"totalPages":  posts.TotalPages,
		"hasMore":     posts.Page < posts.TotalPages,
		"queryParams": queryParams,
		"year":        time.Now().Year(),
	})
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
		Status:   "published",
		TagNames: tags,
		Page:     page,
		PerPage:  6,
	}

	posts, err := a.posts.List(filter)
	if err != nil {
		c.String(http.StatusInternalServerError, "")
		return
	}

	hasMore := page < posts.TotalPages

	c.HTML(http.StatusOK, "post_cards.html", gin.H{
		"posts":       posts.Posts,
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

	post, err := a.posts.Get(uint(id))
	if err != nil || post.Status != "published" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	htmlContent, err := renderMarkdown(post.Content)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "post_detail.html", gin.H{
			"title": "文章详情",
			"error": "渲染内容失败",
			"year":  time.Now().Year(),
		})
		return
	}

	c.HTML(http.StatusOK, "post_detail.html", gin.H{
		"title":   post.Title,
		"post":    post,
		"content": htmlContent,
		"year":    time.Now().Year(),
	})
}

// ShowTagArchive lists tags and related published post counts.
func (a *API) ShowTagArchive(c *gin.Context) {
	stats := a.buildTagStats()

	c.HTML(http.StatusOK, "tag_list.html", gin.H{
		"title": "标签",
		"tags":  stats,
		"year":  time.Now().Year(),
	})
}

// ShowAbout renders the dynamic about page.
func (a *API) ShowAbout(c *gin.Context) {
	page, err := a.pages.GetBySlug("about")
	if err != nil {
		c.HTML(http.StatusOK, "about.html", gin.H{
			"title": "关于",
			"page": gin.H{
				"Title":   "关于我",
				"Summary": "保持好奇心，持续输出价值。",
			},
			"content": template.HTML("<p class=\"text-sm text-slate-600\">暂无简介，稍后再来看看。</p>"),
			"year":    time.Now().Year(),
		})
		return
	}

	htmlContent, err := renderMarkdown(page.Content)
	if err != nil {
		htmlContent = template.HTML("<p class=\"text-sm text-slate-600\">内容暂时无法展示。</p>")
	}

	c.HTML(http.StatusOK, "about.html", gin.H{
		"title":   page.Title,
		"page":    page,
		"content": htmlContent,
		"year":    time.Now().Year(),
	})
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
	tagsWithPosts, err := a.tags.ListWithPosts()
	if err != nil {
		return nil
	}

	stats := make([]tagStat, 0, len(tagsWithPosts))
	for _, tag := range tagsWithPosts {
		count := 0
		for _, post := range tag.Posts {
			if post.Status == "published" {
				count++
			}
		}
		if count > 0 {
			stats = append(stats, tagStat{Name: tag.Name, Count: count})
		}
	}

	return stats
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
