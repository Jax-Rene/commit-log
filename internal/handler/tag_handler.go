package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
)

type tagRequest struct {
	Name string `json:"name" binding:"required"`
}

// GetTags 获取标签列表
func GetTags(c *gin.Context) {
	svc := service.NewTagService(db.DB)
	tags, err := svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取标签列表失败"})
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
func CreateTag(c *gin.Context) {
	var req tagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签名称不能为空"})
		return
	}

	svc := service.NewTagService(db.DB)
	tag, err := svc.Create(req.Name)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTagExists):
			c.JSON(http.StatusBadRequest, gin.H{"error": "标签已存在"})
		case errors.Is(err, service.ErrTagNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建标签失败"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "标签创建成功", "tag": tag})
}

// UpdateTag 更新标签
func UpdateTag(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的标签ID"})
		return
	}

	var req tagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签名称不能为空"})
		return
	}

	svc := service.NewTagService(db.DB)
	tag, err := svc.Update(uint(id), req.Name)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTagExists):
			c.JSON(http.StatusBadRequest, gin.H{"error": "标签名已存在"})
		case errors.Is(err, service.ErrTagNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新标签失败"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "标签更新成功", "tag": tag})
}

// DeleteTag 删除标签
func DeleteTag(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的标签ID"})
		return
	}

	svc := service.NewTagService(db.DB)
	if err := svc.Delete(uint(id)); err != nil {
		switch {
		case errors.Is(err, service.ErrTagInUse):
			c.JSON(http.StatusBadRequest, gin.H{"error": "标签正在被文章使用，无法删除"})
		case errors.Is(err, service.ErrTagNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除标签失败"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "标签删除成功"})
}
