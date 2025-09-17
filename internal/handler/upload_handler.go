package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UploadImage 处理图片上传请求
func UploadImage(c *gin.Context) {
	// 获取上传的文件
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传的图片", "success": 0})
		return
	}

	// 检查文件类型
	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只允许上传图片文件", "success": 0})
		return
	}

	// 创建上传目录
	uploadDir := "./web/static/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建上传目录失败", "success": 0})
		return
	}

	// 生成唯一文件名
	ext := filepath.Ext(file.Filename)
	newFilename := fmt.Sprintf("%s-%s%s", time.Now().Format("20060102"), uuid.New().String(), ext)
	filePath := filepath.Join(uploadDir, newFilename)

	// 保存文件
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败", "success": 0})
		return
	}

	// 返回文件URL - 符合EasyMDE的预期格式
	fileURL := fmt.Sprintf("/static/uploads/%s", newFilename)
	c.JSON(http.StatusOK, gin.H{
		"success": 1,
		"message": "上传成功",
		"data": gin.H{
			"filePath": fileURL,
			"url":      fileURL,
		},
	})
}