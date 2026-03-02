package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
)

type postTemplatePayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Summary     string `json:"summary"`
	Visibility  string `json:"visibility"`
	CoverURL    string `json:"cover_url"`
	CoverWidth  int    `json:"cover_width"`
	CoverHeight int    `json:"cover_height"`
	TagIDs      []uint `json:"tag_ids"`
}

type createFromTemplatePayload struct {
	TemplateID     uint   `json:"template_id"`
	Title          string `json:"title"`
	DraftSessionID string `json:"draft_session_id"`
}

func (p postTemplatePayload) toInput() service.PostTemplateInput {
	return service.PostTemplateInput{
		Name:        p.Name,
		Description: p.Description,
		Content:     p.Content,
		Summary:     p.Summary,
		Visibility:  p.Visibility,
		CoverURL:    p.CoverURL,
		CoverWidth:  p.CoverWidth,
		CoverHeight: p.CoverHeight,
		TagIDs:      p.TagIDs,
	}
}

// ShowPostTemplateManagement 渲染模板管理页面。
func (a *API) ShowPostTemplateManagement(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	page := 1
	if p, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && p > 0 {
		page = p
	}

	data := gin.H{
		"title":     "模板管理",
		"keyword":   keyword,
		"templates": []db.PostTemplate{},
		"allTags":   []db.Tag{},
		"error":     "",
	}

	if tags, err := a.tags.List(); err == nil {
		data["allTags"] = tags
	} else {
		c.Error(err)
	}

	result, err := a.templates.List(service.TemplateFilter{
		Keyword: keyword,
		Page:    page,
		PerPage: 50,
	})
	if err != nil {
		data["error"] = "模板加载失败，请稍后重试"
		a.renderHTML(c, http.StatusOK, "post_template_manage.html", data)
		return
	}

	data["templates"] = result.Templates
	data["page"] = result.Page
	data["total"] = result.Total
	data["totalPages"] = result.TotalPages
	a.renderHTML(c, http.StatusOK, "post_template_manage.html", data)
}

// ListPostTemplates 获取模板列表。
func (a *API) ListPostTemplates(c *gin.Context) {
	page := 1
	if p, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && p > 0 {
		page = p
	}
	perPage := 20
	if p, err := strconv.Atoi(c.DefaultQuery("per_page", "20")); err == nil && p > 0 {
		perPage = p
	}

	result, err := a.templates.List(service.TemplateFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Page:    page,
		PerPage: perPage,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取模板列表失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"templates":   result.Templates,
		"total":       result.Total,
		"page":        result.Page,
		"per_page":    result.PerPage,
		"total_pages": result.TotalPages,
	})
}

// GetPostTemplate 获取模板详情。
func (a *API) GetPostTemplate(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的模板ID")
		return
	}

	template, err := a.templates.Get(id)
	if err != nil {
		if errors.Is(err, service.ErrTemplateNotFound) {
			respondError(c, http.StatusNotFound, "模板不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "获取模板详情失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": template})
}

// CreatePostTemplate 创建模板。
func (a *API) CreatePostTemplate(c *gin.Context) {
	var payload postTemplatePayload
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	template, err := a.templates.Create(payload.toInput())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTagNotFound):
			respondError(c, http.StatusBadRequest, "部分标签不存在")
		case errors.Is(err, service.ErrVisibilityInvalid):
			respondError(c, http.StatusBadRequest, "模板可见度不合法")
		default:
			respondError(c, http.StatusInternalServerError, "创建模板失败")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "模板创建成功", "template": template})
}

// UpdatePostTemplate 更新模板。
func (a *API) UpdatePostTemplate(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的模板ID")
		return
	}

	var payload postTemplatePayload
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	template, err := a.templates.Update(id, payload.toInput())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTemplateNotFound):
			respondError(c, http.StatusNotFound, "模板不存在")
		case errors.Is(err, service.ErrTagNotFound):
			respondError(c, http.StatusBadRequest, "部分标签不存在")
		case errors.Is(err, service.ErrVisibilityInvalid):
			respondError(c, http.StatusBadRequest, "模板可见度不合法")
		default:
			respondError(c, http.StatusInternalServerError, "更新模板失败")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "模板更新成功", "template": template})
}

// DeletePostTemplate 删除模板。
func (a *API) DeletePostTemplate(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的模板ID")
		return
	}

	if err := a.templates.Delete(id); err != nil {
		if errors.Is(err, service.ErrTemplateNotFound) {
			respondError(c, http.StatusNotFound, "模板不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "删除模板失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "模板删除成功"})
}

// CreatePostFromTemplate 根据模板创建文章草稿。
func (a *API) CreatePostFromTemplate(c *gin.Context) {
	var payload createFromTemplatePayload
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	post, err := a.posts.CreateFromTemplate(service.CreatePostFromTemplateInput{
		TemplateID:     payload.TemplateID,
		Title:          payload.Title,
		UserID:         a.currentUserID(c),
		DraftSessionID: payload.DraftSessionID,
		Now:            time.Now(),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTemplateNotFound):
			respondError(c, http.StatusNotFound, "模板不存在")
		case errors.Is(err, service.ErrVisibilityInvalid):
			respondError(c, http.StatusBadRequest, "模板可见度不合法")
		default:
			respondError(c, http.StatusInternalServerError, "从模板创建文章失败")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "文章创建成功",
		"post":    post,
	})
}
