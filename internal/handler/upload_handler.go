package handler

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "image/gif"
)

// UploadImage 处理图片上传请求
func UploadImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传的图片", "success": 0})
		return
	}

	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只允许上传图片文件", "success": 0})
		return
	}

	uploadDir := "./web/static/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建上传目录失败", "success": 0})
		return
	}

	ext := filepath.Ext(file.Filename)
	newFilename := fmt.Sprintf("%s-%s%s", time.Now().Format("20060102"), uuid.New().String(), ext)
	filePath := filepath.Join(uploadDir, newFilename)

	data, cfg, format, err := readImageData(file)
	if err != nil {
		// 回退：直接保存原文件
		if err := c.SaveUploadedFile(file, filePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败", "success": 0})
			return
		}
		width, height, dimErr := imageDimensions(filePath)
		if dimErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取图片信息失败", "success": 0})
			return
		}
		respondSuccess(c, filePath, width, height)
		return
	}

	if err := compressAndSave(filePath, data, format); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩图片失败", "success": 0})
		return
	}

	respondSuccess(c, filePath, cfg.Width, cfg.Height)
}

func readImageData(file *multipart.FileHeader) ([]byte, image.Config, string, error) {
	src, err := file.Open()
	if err != nil {
		return nil, image.Config{}, "", err
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, image.Config{}, "", err
	}

	reader := bytes.NewReader(data)
	cfg, format, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, image.Config{}, "", err
	}

	return data, cfg, format, nil
}

func compressAndSave(path string, data []byte, format string) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return jpeg.Encode(out, img, &jpeg.Options{Quality: 78})
	case "png":
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		return encoder.Encode(out, img)
	default:
		_, err = out.Write(data)
		return err
	}
}

func imageDimensions(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	img, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	return img.Width, img.Height, nil
}

func respondSuccess(c *gin.Context, path string, width, height int) {
	rel, err := filepath.Rel("./web", path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	fileURL := "/" + strings.TrimLeft(rel, "/")
	c.JSON(http.StatusOK, gin.H{
		"success": 1,
		"message": "上传成功",
		"data": gin.H{
			"filePath": fileURL,
			"url":      fileURL,
			"width":    width,
			"height":   height,
		},
	})
}
