package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const defaultUserID = 1

type postPayload struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	Summary     string `json:"summary"`
	TagIDs      []uint `json:"tag_ids"`
	CoverURL    string `json:"cover_url"`
	CoverWidth  int    `json:"cover_width"`
	CoverHeight int    `json:"cover_height"`
}

func (p postPayload) toInput(userID uint) service.PostInput {
	return service.PostInput{
		Title:       p.Title,
		Content:     p.Content,
		Summary:     p.Summary,
		TagIDs:      p.TagIDs,
		UserID:      userID,
		CoverURL:    p.CoverURL,
		CoverWidth:  p.CoverWidth,
		CoverHeight: p.CoverHeight,
	}
}

// GetPosts 获取文章列表
func (a *API) GetPosts(c *gin.Context) {
	posts, err := a.posts.ListAll()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取文章列表失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// GetPost 获取单篇文章
func (a *API) GetPost(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的文章ID")
		return
	}

	post, err := a.posts.Get(id)
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			respondError(c, http.StatusNotFound, "文章不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "获取文章失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": post})
}

// CreatePost 创建新文章
func (a *API) CreatePost(c *gin.Context) {
	var payload postPayload
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	post, err := a.posts.Create(payload.toInput(a.currentUserID(c)))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTagNotFound):
			respondError(c, http.StatusBadRequest, "部分标签不存在")
		case errors.Is(err, service.ErrCoverRequired):
			respondError(c, http.StatusBadRequest, "请上传文章封面")
		case errors.Is(err, service.ErrCoverInvalid):
			respondError(c, http.StatusBadRequest, "封面尺寸无效")
		default:
			respondError(c, http.StatusInternalServerError, "创建文章失败")
		}
		return
	}

	notices, warnings := a.maybeGenerateSummary(c, post)

	response := gin.H{"message": "文章创建成功", "post": post}
	if len(notices) > 0 {
		response["notices"] = notices
	}
	if len(warnings) > 0 {
		response["warnings"] = warnings
	}

	c.JSON(http.StatusOK, response)
}

// UpdatePost 更新文章
func (a *API) UpdatePost(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的文章ID")
		return
	}

	var payload postPayload
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	post, err := a.posts.Update(id, payload.toInput(a.currentUserID(c)))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPostNotFound):
			respondError(c, http.StatusNotFound, "文章不存在")
		case errors.Is(err, service.ErrTagNotFound):
			respondError(c, http.StatusBadRequest, "部分标签不存在")
		case errors.Is(err, service.ErrCoverRequired):
			respondError(c, http.StatusBadRequest, "请上传文章封面")
		case errors.Is(err, service.ErrCoverInvalid):
			respondError(c, http.StatusBadRequest, "封面尺寸无效")
		default:
			respondError(c, http.StatusInternalServerError, "更新文章失败")
		}
		return
	}

	notices, warnings := a.maybeGenerateSummary(c, post)
	response := gin.H{"message": "草稿更新成功", "post": post}
	if len(notices) > 0 {
		response["notices"] = notices
	}
	if len(warnings) > 0 {
		response["warnings"] = warnings
	}

	c.JSON(http.StatusOK, response)
}

// PublishPost 发布文章并生成快照
func (a *API) PublishPost(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的文章ID")
		return
	}

	publication, err := a.posts.Publish(id, a.currentUserID(c))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPostNotFound):
			respondError(c, http.StatusNotFound, "文章不存在")
			return
		case errors.Is(err, service.ErrCoverRequired):
			respondError(c, http.StatusBadRequest, "请上传文章封面后再发布")
			return
		case errors.Is(err, service.ErrCoverInvalid):
			respondError(c, http.StatusBadRequest, "封面尺寸无效，请重新裁剪")
			return
		case errors.Is(err, service.ErrInvalidPublishState):
			respondError(c, http.StatusBadRequest, "请完善标题与正文内容后再发布")
			return
		default:
			respondError(c, http.StatusInternalServerError, "发布文章失败")
			return
		}
	}

	post, err := a.posts.Get(id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "发布完成但刷新文章信息失败，请手动刷新页面")
		return
	}

	notices, warnings := a.maybeGenerateSummary(c, post)

	if refreshed, refreshErr := a.posts.LatestPublication(id); refreshErr == nil {
		publication = refreshed
	} else {
		c.Error(refreshErr)
	}

	response := gin.H{
		"message":     "文章发布成功",
		"publication": publication,
		"post":        post,
	}
	if len(notices) > 0 {
		response["notices"] = notices
	}
	if len(warnings) > 0 {
		response["warnings"] = warnings
	}

	c.JSON(http.StatusOK, response)
}

// DeletePost 删除文章
func (a *API) DeletePost(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的文章ID")
		return
	}

	if err := a.posts.Delete(id); err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			respondError(c, http.StatusNotFound, "文章不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "删除文章失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文章删除成功"})
}

// GeneratePostSummary 使用已配置的 AI 服务生成文章摘要，返回预览内容供人工确认。
func (a *API) GeneratePostSummary(c *gin.Context) {
	if a.summaries == nil {
		respondError(c, http.StatusServiceUnavailable, "未配置 AI 摘要服务，请先在系统设置中完成配置")
		return
	}

	var payload struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	title := strings.TrimSpace(payload.Title)
	content := strings.TrimSpace(payload.Content)
	if title == "" && content == "" {
		respondError(c, http.StatusBadRequest, "请至少提供文章标题或正文内容")
		return
	}

	result, err := a.summaries.GenerateSummary(c.Request.Context(), service.SummaryInput{
		Title:   title,
		Content: content,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAIAPIKeyMissing):
			respondError(c, http.StatusBadRequest, "请在系统设置中配置有效的 AI API Key 后再试")
		default:
			respondError(c, http.StatusBadGateway, fmt.Sprintf("生成摘要失败：%v", err))
		}
		return
	}

	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		respondError(c, http.StatusBadGateway, "AI 摘要服务未返回内容，请稍后重试")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": summary,
		"usage": gin.H{
			"prompt_tokens":     result.PromptTokens,
			"completion_tokens": result.CompletionTokens,
		},
	})
}

// OptimizePostContent 使用 AI 对文章正文进行全文优化，返回优化后的 Markdown。
func (a *API) OptimizePostContent(c *gin.Context) {
	if a.optimizer == nil {
		respondError(c, http.StatusServiceUnavailable, "未配置 AI 全文优化服务，请先在系统设置中完成配置")
		return
	}

	var payload struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	content := strings.TrimSpace(payload.Content)
	if content == "" {
		respondError(c, http.StatusBadRequest, "请先填写文章正文后再尝试全文优化")
		return
	}

	result, err := a.optimizer.OptimizeContent(c.Request.Context(), service.ContentOptimizationInput{
		Content: content,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAIAPIKeyMissing):
			respondError(c, http.StatusBadRequest, "请在系统设置中配置有效的 AI API Key 后再试")
		case errors.Is(err, service.ErrOptimizationEmpty):
			respondError(c, http.StatusBadGateway, "AI 全文优化未返回内容，请稍后重试")
		default:
			respondError(c, http.StatusBadGateway, fmt.Sprintf("全文优化失败：%v", err))
		}
		return
	}

	optimized := strings.TrimSpace(result.Content)
	if optimized == "" {
		respondError(c, http.StatusBadGateway, "AI 全文优化未返回内容，请稍后重试")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"optimized_content": optimized,
		"usage": gin.H{
			"prompt_tokens":     result.PromptTokens,
			"completion_tokens": result.CompletionTokens,
		},
	})
}

// RewritePostSelection 使用 AI 对选中片段进行定向改写。
func (a *API) RewritePostSelection(c *gin.Context) {
	if a.snippetRewriter == nil {
		respondError(c, http.StatusServiceUnavailable, "未配置 AI Chat 能力，请先在系统设置中完成配置")
		return
	}

	var payload struct {
		Selection   string `json:"selection"`
		Instruction string `json:"instruction"`
		Context     string `json:"context"`
	}
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	selection := strings.TrimSpace(payload.Selection)
	if selection == "" {
		respondError(c, http.StatusBadRequest, "请先选择需要改写的段落后再试")
		return
	}

	instruction := strings.TrimSpace(payload.Instruction)
	if instruction == "" {
		respondError(c, http.StatusBadRequest, "请输入改写指令")
		return
	}

	contextText := strings.TrimSpace(payload.Context)

	result, err := a.snippetRewriter.RewriteSnippet(c.Request.Context(), service.SnippetRewriteInput{
		Selection:   selection,
		Instruction: instruction,
		Context:     contextText,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAIAPIKeyMissing):
			respondError(c, http.StatusBadRequest, "请在系统设置中配置有效的 AI API Key 后再试")
		case errors.Is(err, service.ErrSnippetRewriteEmpty):
			respondError(c, http.StatusBadGateway, "AI Chat 未返回内容，请稍后重试")
		default:
			respondError(c, http.StatusBadGateway, fmt.Sprintf("AI Chat 请求失败：%v", err))
		}
		return
	}

	rewritten := strings.TrimSpace(result.Content)
	if rewritten == "" {
		respondError(c, http.StatusBadGateway, "AI Chat 未返回内容，请稍后重试")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": rewritten,
		"usage": gin.H{
			"prompt_tokens":     result.PromptTokens,
			"completion_tokens": result.CompletionTokens,
		},
	})
}

func (a *API) maybeGenerateSummary(c *gin.Context, post *db.Post) (notices, warnings []string) {
	if post == nil {
		return nil, nil
	}
	if strings.TrimSpace(post.Summary) != "" {
		return nil, nil
	}
	if !strings.EqualFold(strings.TrimSpace(post.Status), "published") {
		return nil, nil
	}
	if a.summaries == nil {
		return nil, []string{"未配置 AI 摘要服务，无法自动生成摘要"}
	}
	result, err := a.summaries.GenerateSummary(c.Request.Context(), service.SummaryInput{
		Title:   post.Title,
		Content: post.Content,
	})
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("自动摘要生成失败：%v", err))
		return nil, warnings
	}
	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		warnings = append(warnings, "自动摘要生成失败：模型未返回内容")
		return nil, warnings
	}
	if err := a.posts.UpdateSummary(post.ID, summary); err != nil {
		warnings = append(warnings, "自动摘要保存失败，请稍后重试")
		return nil, warnings
	}
	post.Summary = summary
	notices = append(notices, "已自动生成文章摘要，可根据需要调整内容")
	return notices, nil
}

// ShowPostList 渲染文章管理列表页面
func (a *API) ShowPostList(c *gin.Context) {
	page := 1
	if p, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && p > 0 {
		page = p
	}

	perPage := 10
	search := c.Query("search")
	status := c.Query("status")
	tagNames := c.QueryArray("tags")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var startPtr, endPtr *time.Time
	if startDate != "" {
		if start, err := time.Parse("2006-01-02", startDate); err == nil {
			startPtr = &start
		}
	}
	if endDate != "" {
		if end, err := time.Parse("2006-01-02", endDate); err == nil {
			end = end.Add(24*time.Hour - time.Second)
			endPtr = &end
		}
	}

	filter := service.PostFilter{
		Search:    search,
		Status:    status,
		TagNames:  tagNames,
		StartDate: startPtr,
		EndDate:   endPtr,
		Page:      page,
		PerPage:   perPage,
	}

	list, err := a.posts.List(filter)
	if err != nil {
		a.renderHTML(c, http.StatusInternalServerError, "post_list.html", gin.H{
			"title": "文章管理",
			"error": "获取文章列表失败",
		})
		return
	}

	statsMap := make(map[uint]*db.PostStatistic)
	var overview service.SiteOverview
	if len(list.Posts) > 0 && a.analytics != nil {
		postIDs := make([]uint, 0, len(list.Posts))
		for _, post := range list.Posts {
			postIDs = append(postIDs, post.ID)
		}

		if statData, statErr := a.analytics.PostStatsMap(postIDs); statErr == nil {
			for id, stat := range statData {
				statsMap[id] = stat
			}
		} else {
			c.Error(statErr)
		}
	}

	if a.analytics != nil {
		if ov, ovErr := a.analytics.Overview(5); ovErr == nil {
			overview = ov
		} else {
			c.Error(ovErr)
		}
	}

	tags, err := a.tags.List()
	if err != nil {
		a.renderHTML(c, http.StatusInternalServerError, "post_list.html", gin.H{
			"title": "文章管理",
			"error": "获取标签信息失败",
		})
		return
	}

	pages := make([]int, 0, list.TotalPages)
	for i := 1; i <= list.TotalPages; i++ {
		pages = append(pages, i)
	}

	params := url.Values{}
	if search != "" {
		params.Set("search", search)
	}
	if status != "" {
		params.Set("status", status)
	}
	if startDate != "" {
		params.Set("start_date", startDate)
	}
	if endDate != "" {
		params.Set("end_date", endDate)
	}
	for _, tag := range tagNames {
		params.Add("tags", tag)
	}

	queryParams := params.Encode()
	if queryParams != "" {
		queryParams = "&" + queryParams
	}

	a.renderHTML(c, http.StatusOK, "post_list.html", gin.H{
		"title":          "文章管理",
		"posts":          list.Posts,
		"allTags":        tags,
		"search":         search,
		"status":         status,
		"tags":           tagNames,
		"startDate":      startDate,
		"endDate":        endDate,
		"page":           list.Page,
		"perPage":        list.PerPage,
		"total":          list.Total,
		"totalPages":     list.TotalPages,
		"publishedCount": list.PublishedCount,
		"draftCount":     list.DraftCount,
		"pages":          pages,
		"queryParams":    queryParams,
		"postStats":      statsMap,
		"overview":       overview,
	})
}

func (a *API) postEditPageData(c *gin.Context) gin.H {
	data := gin.H{
		"title": "创建文章",
	}

	if idParam := c.Param("id"); idParam != "" {
		if id, err := strconv.ParseUint(idParam, 10, 32); err == nil {
			post, err := a.posts.Get(uint(id))
			if err == nil {
				data["title"] = "编辑文章"
				data["post"] = post
				if publication, pubErr := a.posts.LatestPublication(post.ID); pubErr == nil {
					data["latestPublication"] = publication
				} else if !errors.Is(pubErr, service.ErrPublicationNotFound) {
					c.Error(pubErr)
				}
			} else if errors.Is(err, service.ErrPostNotFound) {
				data["error"] = "文章不存在"
			} else {
				data["error"] = "加载文章失败"
			}
		}
	}

	return data
}

// ShowPostEdit 渲染文章编辑页面（Milkdown 版本）
func (a *API) ShowPostEdit(c *gin.Context) {
	a.renderHTML(c, http.StatusOK, "post_edit.html", a.postEditPageData(c))
}

func (a *API) currentUserID(c *gin.Context) uint {
	if _, exists := c.Get(sessions.DefaultKey); !exists {
		return defaultUserID
	}

	session := sessions.Default(c)
	if session != nil {
		if val := session.Get("user_id"); val != nil {
			switch v := val.(type) {
			case uint:
				if v > 0 {
					return v
				}
			case int:
				if v > 0 {
					return uint(v)
				}
			case int64:
				if v > 0 {
					return uint(v)
				}
			case string:
				if parsed, err := strconv.ParseUint(v, 10, 32); err == nil && parsed > 0 {
					return uint(parsed)
				}
			}
		}
	}
	return defaultUserID
}
