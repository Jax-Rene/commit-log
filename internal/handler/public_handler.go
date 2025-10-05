package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cmp"
	"slices"

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
	sanitizer = bluemonday.UGCPolicy()
)

const (
	visitorCookieName   = "cl_visitor_id"
	visitorCookieMaxAge = 365 * 24 * 60 * 60
)

type tagStat struct {
	Name  string
	Count int
}

// aboutHeatmapHabit 用于在关于页渲染习惯图例
type aboutHeatmapHabit struct {
	ID      uint
	Name    string
	TypeTag string
}

// aboutHeatmapSummary 汇总热力图相关统计数据
type aboutHeatmapSummary struct {
	TotalLogs  int
	ActiveDays int
	HabitCount int
}

// aboutHeatmapDay 表示热力图中的单日信息
type aboutHeatmapDay struct {
	Date    string
	Count   int
	Class   string
	Muted   bool
	Tooltip string
}

// aboutHeatmapWeek 以周为单位组织热力图列
type aboutHeatmapWeek struct {
	MonthLabel string
	Days       []aboutHeatmapDay
}

// aboutHeatmapData 封装关于页需要的整体热力图数据
type aboutHeatmapData struct {
	RangeStart  string
	RangeEnd    string
	GeneratedAt string
	Summary     aboutHeatmapSummary
	Weeks       []aboutHeatmapWeek
	Habits      []aboutHeatmapHabit
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
		a.renderHTML(c, http.StatusInternalServerError, "home.html", gin.H{
			"title": "首页",
			"error": "获取文章失败",
			"year":  time.Now().Year(),
		})
		return
	}

	tagOptions := a.buildTagStats()

	queryParams := buildQueryParams(search, tags)

	a.renderHTML(c, http.StatusOK, "home.html", gin.H{
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

	a.renderHTML(c, http.StatusOK, "post_cards.html", gin.H{
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

	visitorID := a.ensureVisitorID(c)

	var (
		pageViews      uint64
		uniqueVisitors uint64
	)

	if a.analytics != nil {
		if stats, recordErr := a.analytics.RecordPostView(post.ID, visitorID, time.Now().UTC()); recordErr == nil {
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

	htmlContent, err := renderMarkdown(post.Content)
	if err != nil {
		a.renderHTML(c, http.StatusInternalServerError, "post_detail.html", gin.H{
			"title":    "文章详情",
			"error":    "渲染内容失败",
			"year":     time.Now().Year(),
			"contacts": contacts,
		})
		return
	}

	a.renderHTML(c, http.StatusOK, "post_detail.html", gin.H{
		"title":          post.Title,
		"post":           post,
		"content":        htmlContent,
		"contacts":       contacts,
		"pageViews":      pageViews,
		"uniqueVisitors": uniqueVisitors,
		"year":           time.Now().Year(),
	})
}

// ShowTagArchive lists tags and related published post counts.
func (a *API) ShowTagArchive(c *gin.Context) {
	stats := a.buildTagStats()

	a.renderHTML(c, http.StatusOK, "tag_list.html", gin.H{
		"title": "标签",
		"tags":  stats,
		"year":  time.Now().Year(),
	})
}

func (a *API) ensureVisitorID(c *gin.Context) string {
	if id, err := c.Cookie(visitorCookieName); err == nil && strings.TrimSpace(id) != "" {
		return id
	}

	visitorID := uuid.NewString()
	secure := c.Request.TLS != nil

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

	contacts, contactErr := a.profiles.ListContacts(false)
	if contactErr != nil {
		contacts = nil
	}

	page, err := a.pages.GetBySlug("about")
	if err != nil {
		a.renderHTML(c, http.StatusOK, "about.html", gin.H{
			"title": "关于",
			"page": gin.H{
				"Title":   "关于我",
				"Summary": "保持好奇心，持续输出价值。",
			},
			"content":  template.HTML("<p class=\"text-sm text-slate-600\">暂无简介，稍后再来看看。</p>"),
			"year":     now.Year(),
			"contacts": contacts,
		})
		return
	}

	htmlContent, err := renderMarkdown(page.Content)
	if err != nil {
		htmlContent = template.HTML("<p class=\"text-sm text-slate-600\">内容暂时无法展示。</p>")
	}

	a.renderHTML(c, http.StatusOK, "about.html", gin.H{
		"title":    page.Title,
		"page":     page,
		"content":  htmlContent,
		"year":     now.Year(),
		"contacts": contacts,
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

func buildAboutHabitHeatmap(entries []service.HabitHeatmapEntry, start, end, generatedAt time.Time) aboutHeatmapData {
	dayHabits := make(map[string][]aboutHeatmapHabit)
	legendMap := make(map[uint]aboutHeatmapHabit)

	for _, entry := range entries {
		dateKey := entry.LogDate.Format("2006-01-02")
		habit := aboutHeatmapHabit{ID: entry.HabitID, Name: entry.HabitName, TypeTag: entry.HabitType}
		dayHabits[dateKey] = append(dayHabits[dateKey], habit)
		if _, exists := legendMap[habit.ID]; !exists {
			legendMap[habit.ID] = habit
		}
	}

	legend := make([]aboutHeatmapHabit, 0, len(legendMap))
	for _, habit := range legendMap {
		legend = append(legend, habit)
	}

	slices.SortFunc(legend, func(a, b aboutHeatmapHabit) int {
		if diff := cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); diff != 0 {
			return diff
		}
		return cmp.Compare(a.ID, b.ID)
	})

	alignedStart := start
	for alignedStart.Weekday() != time.Monday {
		alignedStart = alignedStart.AddDate(0, 0, -1)
	}
	alignedEnd := end
	for alignedEnd.Weekday() != time.Sunday {
		alignedEnd = alignedEnd.AddDate(0, 0, 1)
	}

	weeks := make([]aboutHeatmapWeek, 0, 60)
	lastMonth := 0

	for weekStart := alignedStart; !weekStart.After(alignedEnd); weekStart = weekStart.AddDate(0, 0, 7) {
		week := aboutHeatmapWeek{Days: make([]aboutHeatmapDay, 0, 7)}
		weekEnd := weekStart.AddDate(0, 0, 6)
		label := ""

		for day := weekStart; !day.After(weekEnd); day = day.AddDate(0, 0, 1) {
			dateKey := day.Format("2006-01-02")
			habits := append([]aboutHeatmapHabit(nil), dayHabits[dateKey]...)
			slices.SortFunc(habits, func(a, b aboutHeatmapHabit) int {
				return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
			})

			names := make([]string, 0, len(habits))
			for _, habit := range habits {
				names = append(names, habit.Name)
			}

			count := len(habits)
			muted := day.Before(start) || day.After(end)

			title := fmt.Sprintf("%s：暂无打卡", dateKey)
			if count > 0 {
				title = fmt.Sprintf("%s：%d 次打卡", dateKey, count)
				if len(names) > 0 {
					title += "\n习惯：" + strings.Join(names, "、")
				}
			}

			week.Days = append(week.Days, aboutHeatmapDay{
				Date:    dateKey,
				Count:   count,
				Class:   colorClassForCount(count),
				Muted:   muted,
				Tooltip: title,
			})

			if !muted && label == "" {
				month := int(day.Month())
				if month != lastMonth {
					label = fmt.Sprintf("%d月", month)
					lastMonth = month
				}
			}
		}

		week.MonthLabel = label
		weeks = append(weeks, week)
	}

	return aboutHeatmapData{
		RangeStart:  start.Format("2006-01-02"),
		RangeEnd:    end.Format("2006-01-02"),
		GeneratedAt: generatedAt.In(time.Local).Format("2006-01-02 15:04"),
		Summary: aboutHeatmapSummary{
			TotalLogs:  len(entries),
			ActiveDays: len(dayHabits),
			HabitCount: len(legend),
		},
		Weeks:  weeks,
		Habits: legend,
	}
}

func colorClassForCount(count int) string {
	switch {
	case count <= 0:
		return "bg-slate-200 dark:bg-slate-700/60"
	case count == 1:
		return "bg-emerald-200 dark:bg-emerald-700/60"
	case count == 2:
		return "bg-emerald-300 dark:bg-emerald-600/70"
	case count <= 4:
		return "bg-emerald-400 dark:bg-emerald-500/80"
	default:
		return "bg-emerald-600 dark:bg-emerald-400/80"
	}
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
