package handler

import (
	"net/http"
	"strconv"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
)

// TagHandler 处理标签相关的请求

// GetTags 获取标签列表
func GetTags(c *gin.Context) {
	var tags []db.Tag
	if err := db.DB.Preload("Posts").Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取标签列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// ShowTagList 渲染标签管理列表页面
func ShowTagList(c *gin.Context) {
	var tags []db.Tag
	if err := db.DB.Preload("Posts").Order("created_at desc").Find(&tags).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "tag_list.html", gin.H{
			"title": "标签管理",
			"error": "获取标签列表失败",
		})
		return
	}

	c.HTML(http.StatusOK, "tag_list.html", gin.H{
		"title": "标签管理",
		"tags":  tags,
	})
}

// CreateTag 创建新标签
func CreateTag(c *gin.Context) {
	var tag db.Tag
	if err := c.ShouldBindJSON(&tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查标签是否已存在
	var existingTag db.Tag
	if err := db.DB.Where("name = ?", tag.Name).First(&existingTag).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签已存在"})
		return
	}

	if err := db.DB.Create(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建标签失败"})
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

	var tag db.Tag
	if err := db.DB.First(&tag, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		return
	}

	if err := c.ShouldBindJSON(&tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查新标签名是否已存在
	var existingTag db.Tag
	if err := db.DB.Where("name = ? AND id != ?", tag.Name, id).First(&existingTag).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签名已存在"})
		return
	}

	if err := db.DB.Save(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新标签失败"})
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

	// 检查标签是否存在
	var tag db.Tag
	if err := db.DB.First(&tag, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		return
	}

	// 检查标签是否被文章使用
	var count int64
	db.DB.Model(&db.Post{}).Joins("JOIN post_tags ON posts.id = post_tags.post_id").Where("post_tags.tag_id = ?", id).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签正在被文章使用，无法删除"})
		return
	}

	if err := db.DB.Delete(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除标签失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "标签删除成功"})
}
