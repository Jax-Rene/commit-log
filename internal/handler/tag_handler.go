package handler

import (
	"errors"
	"net/http"

	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
)

type tagRequest struct {
	Name string `json:"name" binding:"required"`
}

// GetTags 获取标签列表
func (a *API) GetTags(c *gin.Context) {
	tags, err := a.tags.List()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取标签列表失败")
		return
	}

	response := make([]gin.H, 0, len(tags))
	for _, tag := range tags {
		response = append(response, gin.H{
			"id":   tag.ID,
			"name": tag.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{"tags": response})
}

// CreateTag 创建新标签
func (a *API) CreateTag(c *gin.Context) {
	var req tagRequest
	if !bindJSON(c, &req, "标签名称不能为空") {
		return
	}

	tag, err := a.tags.Create(req.Name)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTagExists):
			respondError(c, http.StatusBadRequest, "标签已存在")
		case errors.Is(err, service.ErrTagNotFound):
			respondError(c, http.StatusNotFound, "标签不存在")
		default:
			respondError(c, http.StatusInternalServerError, "创建标签失败")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "标签创建成功", "tag": tag})
}

// UpdateTag 更新标签
func (a *API) UpdateTag(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的标签ID")
		return
	}

	var req tagRequest
	if !bindJSON(c, &req, "标签名称不能为空") {
		return
	}

	tag, err := a.tags.Update(id, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTagExists):
			respondError(c, http.StatusBadRequest, "标签名已存在")
		case errors.Is(err, service.ErrTagNotFound):
			respondError(c, http.StatusNotFound, "标签不存在")
		default:
			respondError(c, http.StatusInternalServerError, "更新标签失败")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "标签更新成功", "tag": tag})
}

// DeleteTag 删除标签
func (a *API) DeleteTag(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的标签ID")
		return
	}

	if err := a.tags.Delete(id); err != nil {
		switch {
		case errors.Is(err, service.ErrTagInUse):
			respondError(c, http.StatusBadRequest, "标签正在被文章使用，无法删除")
		case errors.Is(err, service.ErrTagNotFound):
			respondError(c, http.StatusNotFound, "标签不存在")
		default:
			respondError(c, http.StatusInternalServerError, "删除标签失败")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "标签删除成功"})
}
