package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
)

const defaultUserID = 1

type postPayload struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
	TagIDs  []uint `json:"tag_ids"`
}

func (p postPayload) toInput() service.PostInput {
	return service.PostInput{
		Title:   p.Title,
		Content: p.Content,
		Summary: p.Summary,
		Status:  p.Status,
		TagIDs:  p.TagIDs,
		UserID:  defaultUserID,
	}
}

// GetPosts 获取文章列表
func GetPosts(c *gin.Context) {
	posts, err := service.NewPostService(db.DB).ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文章列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// GetPost 获取单篇文章
func GetPost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	post, err := service.NewPostService(db.DB).Get(uint(id))
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文章失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": post})
}

// CreatePost 创建新文章
func CreatePost(c *gin.Context) {
	var payload postPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不合法"})
		return
	}

	post, err := service.NewPostService(db.DB).Create(payload.toInput())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTagNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": "部分标签不存在"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文章失败"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文章创建成功", "post": post})
}

// UpdatePost 更新文章
func UpdatePost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	var payload postPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不合法"})
		return
	}

	post, err := service.NewPostService(db.DB).Update(uint(id), payload.toInput())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPostNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		case errors.Is(err, service.ErrTagNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": "部分标签不存在"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新文章失败"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文章更新成功", "post": post})
}

// DeletePost 删除文章
func DeletePost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	if err := service.NewPostService(db.DB).Delete(uint(id)); err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文章失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文章删除成功"})
}

// ShowPostList 渲染文章管理列表页面
func ShowPostList(c *gin.Context) {
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

	postService := service.NewPostService(db.DB)
	list, err := postService.List(filter)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "post_list.html", gin.H{
			"title": "文章管理",
			"error": "获取文章列表失败",
		})
		return
	}

	tags, err := service.NewTagService(db.DB).List()
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
func ShowPostEdit(c *gin.Context) {
	data := gin.H{
		"title": "创建文章",
	}

	if idParam := c.Param("id"); idParam != "" {
		if id, err := strconv.ParseUint(idParam, 10, 32); err == nil {
			post, err := service.NewPostService(db.DB).Get(uint(id))
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
