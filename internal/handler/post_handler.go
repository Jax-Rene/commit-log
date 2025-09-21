package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/commitlog/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const defaultUserID = 1

type postPayload struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
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
		Status:      p.Status,
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

	c.JSON(http.StatusOK, gin.H{"message": "文章创建成功", "post": post})
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

	c.JSON(http.StatusOK, gin.H{"message": "文章更新成功", "post": post})
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
		c.HTML(http.StatusInternalServerError, "post_list.html", gin.H{
			"title": "文章管理",
			"error": "获取文章列表失败",
		})
		return
	}

	tags, err := a.tags.List()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "post_list.html", gin.H{
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

	c.HTML(http.StatusOK, "post_list.html", gin.H{
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
	})
}

// ShowPostEdit 渲染文章编辑页面
func (a *API) ShowPostEdit(c *gin.Context) {
	data := gin.H{
		"title": "创建文章",
	}

	if idParam := c.Param("id"); idParam != "" {
		if id, err := strconv.ParseUint(idParam, 10, 32); err == nil {
			post, err := a.posts.Get(uint(id))
			if err == nil {
				data["title"] = "编辑文章"
				data["post"] = post
			} else if errors.Is(err, service.ErrPostNotFound) {
				data["error"] = "文章不存在"
			} else {
				data["error"] = "加载文章失败"
			}
		}
	}

	c.HTML(http.StatusOK, "post_edit.html", data)
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
