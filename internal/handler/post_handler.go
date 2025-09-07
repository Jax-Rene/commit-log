package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
)

// PostHandler 处理文章相关的请求

// GetPosts 获取文章列表
func GetPosts(c *gin.Context) {
	var posts []db.Post
	if err := db.DB.Preload("Tags").Order("created_at desc").Find(&posts).Error; err != nil {
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

	var post db.Post
	if err := db.DB.Preload("Tags").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": post})
}

// CreatePost 创建新文章
func CreatePost(c *gin.Context) {
	var post db.Post
	if err := c.ShouldBindJSON(&post); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()

	if err := db.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文章失败"})
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

	var post db.Post
	if err := db.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	if err := c.ShouldBindJSON(&post); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post.UpdatedAt = time.Now()
	if err := db.DB.Save(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新文章失败"})
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

	if err := db.DB.Delete(&db.Post{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文章失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文章删除成功"})
}

// ShowPostList 渲染文章管理列表页面
func ShowPostList(c *gin.Context) {
	c.HTML(http.StatusOK, "post_list.html", gin.H{
		"title": "文章管理",
	})
}

// ShowPostEdit 渲染文章编辑页面
func ShowPostEdit(c *gin.Context) {
	id := c.Param("id")
	
	data := gin.H{
		"title": "编辑文章",
	}
	
	if id != "" {
		// 编辑现有文章
		var post db.Post
		if err := db.DB.Preload("Tags").First(&post, id).Error; err == nil {
			data["post"] = post
		}
	} else {
		// 创建新文章
		data["title"] = "创建文章"
	}
	
	c.HTML(http.StatusOK, "post_edit.html", data)
}