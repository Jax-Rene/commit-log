package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
)

type aboutPayload struct {
	Content string `json:"content"`
}

// ShowAboutEditor renders the admin editor for the about page.
func (a *API) ShowAboutEditor(c *gin.Context) {
	page, err := a.pages.GetBySlug("about")
	if err != nil {
		if !errors.Is(err, service.ErrPageNotFound) {
			a.renderHTML(c, http.StatusInternalServerError, "about_edit.html", gin.H{
				"title": "About Me",
				"error": "加载关于页面失败，请稍后再试",
			})
			return
		}
	}

	var content string
	var updatedAt string
	if page != nil {
		content = page.Content
		if !page.UpdatedAt.IsZero() {
			updatedAt = page.UpdatedAt.In(time.Local).Format("2006-01-02 15:04")
		}
	}

	a.renderHTML(c, http.StatusOK, "about_edit.html", gin.H{
		"title":     "About Me",
		"content":   content,
		"updatedAt": updatedAt,
	})
}

// UpdateAboutPage saves the markdown content for the about page.
func (a *API) UpdateAboutPage(c *gin.Context) {
	var payload aboutPayload
	if !bindJSON(c, &payload, "内容格式不正确") {
		return
	}

	page, err := a.pages.SaveAboutPage(payload.Content)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPageContentMissing):
			c.JSON(http.StatusBadRequest, gin.H{"error": "请填写关于页面内容"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败，请稍后重试"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "关于页面已更新",
		"page": gin.H{
			"content":   page.Content,
			"updatedAt": page.UpdatedAt.In(time.Local).Format("2006-01-02 15:04"),
		},
	})
}
