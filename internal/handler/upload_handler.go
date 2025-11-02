package handler

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/image/draw"
	_ "image/gif"
)

const (
	maxUploadBytes    = 20 << 20 // 20MB
	maxImageDimension = 3840     // 限制边长为 4K
	jpegQuality       = 82       // 输出 JPG 质量
	sampleGrid        = 64       // 检测透明像素的采样网格
)

var errImageTooLarge = errors.New("uploaded image exceeds allowed size")

// UploadImage 处理图片上传请求
func (a *API) UploadImage(c *gin.Context) {
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

	uploadDir := a.uploadDir
	if strings.TrimSpace(uploadDir) == "" {
		uploadDir = "web/static/uploads"
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建上传目录失败", "success": 0})
		return
	}

	originalExt := normalizeExt(filepath.Ext(file.Filename))
	baseName := fmt.Sprintf("%s-%s", time.Now().Format("20060102"), uuid.New().String())

	processed, err := processUploadedImage(file)
	if err != nil {
		if errors.Is(err, errImageTooLarge) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "图片体积超过限制，请控制在 20MB 以内", "success": 0})
			return
		}
		filePath := filepath.Join(uploadDir, baseName+resolveExtFromFormat("", originalExt))
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
		respondSuccess(c, filePath, width, height, uploadDir, a.uploadURL)
		return
	}

	filePath := filepath.Join(uploadDir, baseName+resolveExtFromFormat(processed.format, originalExt))
	if err := saveProcessedImage(filePath, processed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩图片失败", "success": 0})
		return
	}

	respondSuccess(c, filePath, processed.width, processed.height, uploadDir, a.uploadURL)
}

type processedImage struct {
	img    image.Image
	width  int
	height int
	format string
}

func saveProcessedImage(path string, img processedImage) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	switch img.format {
	case "jpeg", "jpg":
		return jpeg.Encode(out, img.img, &jpeg.Options{Quality: jpegQuality})
	case "png":
		encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
		return encoder.Encode(out, img.img)
	default:
		return fmt.Errorf("unsupported image format: %s", img.format)
	}
}

func processUploadedImage(file *multipart.FileHeader) (processedImage, error) {
	img, format, err := decodeUploadedImage(file)
	if err != nil {
		return processedImage{}, err
	}

	img = resizeIfNeeded(img, maxImageDimension)
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	outputFormat := normalizeOutputFormat(format, img)

	return processedImage{
		img:    img,
		width:  width,
		height: height,
		format: outputFormat,
	}, nil
}

func decodeUploadedImage(file *multipart.FileHeader) (image.Image, string, error) {
	src, err := file.Open()
	if err != nil {
		return nil, "", fmt.Errorf("open upload failed: %w", err)
	}
	defer src.Close()

	data, err := readWithLimit(src, maxUploadBytes)
	if err != nil {
		return nil, "", err
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode image failed: %w", err)
	}

	return img, format, nil
}

func readWithLimit(r io.Reader, limit int64) ([]byte, error) {
	limited := &io.LimitedReader{R: r, N: limit + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read image failed: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, errImageTooLarge
	}
	return data, nil
}

func resizeIfNeeded(src image.Image, maxSide int) image.Image {
	if maxSide <= 0 {
		return src
	}

	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= maxSide && height <= maxSide {
		return src
	}

	scale := float64(maxSide) / float64(width)
	if height > width {
		scale = float64(maxSide) / float64(height)
	}

	newWidth := int(math.Round(float64(width) * scale))
	newHeight := int(math.Round(float64(height) * scale))
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}

func normalizeOutputFormat(format string, img image.Image) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return "jpeg"
	case "png":
		if hasVisibleAlpha(img) {
			return "png"
		}
		return "jpeg"
	default:
		return "jpeg"
	}
}

func hasVisibleAlpha(img image.Image) bool {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width == 0 || height == 0 {
		return false
	}

	stepX := width / sampleGrid
	if stepX < 1 {
		stepX = 1
	}
	stepY := height / sampleGrid
	if stepY < 1 {
		stepY = 1
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha < 0xffff {
				return true
			}
		}
	}

	_, _, _, alpha := img.At(bounds.Max.X-1, bounds.Max.Y-1).RGBA()
	return alpha < 0xffff
}

func normalizeExt(ext string) string {
	trimmed := strings.TrimSpace(strings.ToLower(ext))
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, ".") {
		trimmed = "." + trimmed
	}
	return trimmed
}

func resolveExtFromFormat(format, fallback string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return ".jpg"
	case "png":
		return ".png"
	}
	if fallback != "" {
		return fallback
	}
	return ".img"
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

func respondSuccess(c *gin.Context, filePath string, width, height int, uploadDir, uploadURL string) {
	var rel string
	if strings.TrimSpace(uploadDir) != "" {
		if r, err := filepath.Rel(uploadDir, filePath); err == nil {
			rel = r
		}
	}
	if strings.TrimSpace(rel) == "" {
		rel = filepath.Base(filePath)
	}
	rel = filepath.ToSlash(rel)
	urlPrefix := strings.TrimRight(uploadURL, "/")
	if urlPrefix == "" {
		urlPrefix = "/uploads"
	}
	fileURL := urlPrefix + "/" + strings.TrimLeft(rel, "/")
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
