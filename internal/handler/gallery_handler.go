package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
)

type galleryPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	ImageWidth  int    `json:"image_width"`
	ImageHeight int    `json:"image_height"`
	Status      string `json:"status"`
	SortOrder   int    `json:"sort_order"`
}

func (p galleryPayload) toInput() service.GalleryInput {
	return service.GalleryInput{
		Title:       p.Title,
		Description: p.Description,
		ImageURL:    p.ImageURL,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
		Status:      p.Status,
		SortOrder:   p.SortOrder,
	}
}

// ShowGalleryManagement renders admin gallery management page.
func (a *API) ShowGalleryManagement(c *gin.Context) {
	items, err := a.galleries.ListAll()
	if err != nil {
		a.renderHTML(c, http.StatusInternalServerError, "gallery_manage.html", gin.H{
			"title": "摄影作品",
			"error": "加载作品集失败",
		})
		return
	}

	a.renderHTML(c, http.StatusOK, "gallery_manage.html", gin.H{
		"title": "摄影作品",
		"items": items,
	})
}

// ListGalleryImages returns all gallery images.
func (a *API) ListGalleryImages(c *gin.Context) {
	items, err := a.galleries.ListAll()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取作品集失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// CreateGalleryImage creates a new gallery image.
func (a *API) CreateGalleryImage(c *gin.Context) {
	var payload galleryPayload
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	item, err := a.galleries.Create(payload.toInput())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrGalleryImageMissing):
			respondError(c, http.StatusBadRequest, "请上传作品图片")
		case errors.Is(err, service.ErrGalleryStatusInvalid):
			respondError(c, http.StatusBadRequest, "作品状态无效")
		default:
			respondError(c, http.StatusInternalServerError, "创建作品失败")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "作品已创建", "item": item})
}

// UpdateGalleryImage updates an existing gallery image.
func (a *API) UpdateGalleryImage(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的作品ID")
		return
	}

	var payload galleryPayload
	if !bindJSON(c, &payload, "请求参数不合法") {
		return
	}

	item, err := a.galleries.Update(id, payload.toInput())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrGalleryNotFound):
			respondError(c, http.StatusNotFound, "作品不存在")
		case errors.Is(err, service.ErrGalleryImageMissing):
			respondError(c, http.StatusBadRequest, "请上传作品图片")
		case errors.Is(err, service.ErrGalleryStatusInvalid):
			respondError(c, http.StatusBadRequest, "作品状态无效")
		default:
			respondError(c, http.StatusInternalServerError, "更新作品失败")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "作品已更新", "item": item})
}

// DeleteGalleryImage removes a gallery image.
func (a *API) DeleteGalleryImage(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的作品ID")
		return
	}

	if err := a.galleries.Delete(id); err != nil {
		switch {
		case errors.Is(err, service.ErrGalleryNotFound):
			respondError(c, http.StatusNotFound, "作品不存在")
		default:
			respondError(c, http.StatusInternalServerError, "删除作品失败")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "作品已删除"})
}

// ShowGallery renders public gallery page.
func (a *API) ShowGallery(c *gin.Context) {
	page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)
	result, err := a.galleries.ListPublished(page, 12)
	if err != nil {
		a.renderHTML(c, http.StatusInternalServerError, "gallery.html", gin.H{
			"title": "摄影作品",
			"error": "加载作品集失败，请稍后重试",
			"year":  time.Now().Year(),
		})
		return
	}

	metaDescription := "摄影作品集，记录镜头下的光影与故事。"
	payload := gin.H{
		"title":           "摄影作品",
		"items":           result.Items,
		"page":            result.Page,
		"totalPages":      result.TotalPages,
		"hasMore":         result.Page < result.TotalPages,
		"canonical":       "/gallery",
		"metaType":        "website",
		"year":            time.Now().Year(),
		"metaDescription": metaDescription,
		"metaKeywords":    []string{"摄影", "作品集", "Gallery"},
	}

	a.renderHTML(c, http.StatusOK, "gallery.html", payload)
}

// LoadMoreGallery returns gallery items for infinite scroll via HTMX.
func (a *API) LoadMoreGallery(c *gin.Context) {
	page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)
	if page < 2 {
		c.String(http.StatusBadRequest, "")
		return
	}

	result, err := a.galleries.ListPublished(page, 12)
	if err != nil {
		c.String(http.StatusInternalServerError, "")
		return
	}

	a.renderHTML(c, http.StatusOK, "gallery_items.html", gin.H{
		"items":    result.Items,
		"hasMore":  result.Page < result.TotalPages,
		"nextPage": page + 1,
	})
}
