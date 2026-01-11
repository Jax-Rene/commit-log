package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/locale"
	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
)

type tagRequest struct {
	Name   string `json:"name"`
	NameZh string `json:"name_zh"`
	NameEn string `json:"name_en"`
}

// ShowTagManagement renders the admin page for managing tags.
func (a *API) ShowTagManagement(c *gin.Context) {
	tags, err := a.tags.List()
	payload := gin.H{
		"title": "标签管理",
		"tags":  tags,
	}
	if err != nil {
		c.Error(err)
		payload["error"] = "加载标签列表失败，请刷新后重试"
	}

	a.renderHTML(c, http.StatusOK, "tag_manage.html", payload)
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
		translations := gin.H{}
		for _, translation := range tag.Translations {
			language := locale.NormalizeLanguage(translation.Language)
			if language == "" {
				continue
			}
			translations[language] = gin.H{
				"name": translation.Name,
			}
		}
		response = append(response, gin.H{
			"id":           tag.ID,
			"name":         tag.Name,
			"created_at":   tag.CreatedAt,
			"updated_at":   tag.UpdatedAt,
			"post_count":   tag.PostCount,
			"translations": translations,
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
	name := strings.TrimSpace(req.Name)
	nameZh := strings.TrimSpace(req.NameZh)
	nameEn := strings.TrimSpace(req.NameEn)
	if name == "" && nameZh == "" && nameEn == "" {
		respondError(c, http.StatusBadRequest, "标签名称不能为空")
		return
	}
	keyName := name
	if keyName == "" {
		if nameZh != "" {
			keyName = nameZh
		} else {
			keyName = nameEn
		}
	}

	tag, err := a.tags.Create(keyName)
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

	requestLang := a.requestLocale(c).Language
	if nameZh != "" {
		_, _ = a.tags.UpsertTranslation(tag.ID, service.TagTranslationInput{
			Language: "zh",
			Name:     nameZh,
		})
	}
	if nameEn != "" {
		_, _ = a.tags.UpsertTranslation(tag.ID, service.TagTranslationInput{
			Language: "en",
			Name:     nameEn,
		})
	}
	if nameZh == "" && nameEn == "" && name != "" {
		_, _ = a.tags.UpsertTranslation(tag.ID, service.TagTranslationInput{
			Language: requestLang,
			Name:     name,
		})
	}

	if refreshed, refreshErr := a.tags.Get(tag.ID); refreshErr == nil {
		tag = refreshed
	} else {
		c.Error(refreshErr)
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

	name := strings.TrimSpace(req.Name)
	nameZh := strings.TrimSpace(req.NameZh)
	nameEn := strings.TrimSpace(req.NameEn)
	requestLang := a.requestLocale(c).Language
	if name == "" && nameZh == "" && nameEn == "" {
		respondError(c, http.StatusBadRequest, "标签名称不能为空")
		return
	}

	// Backward-compatible: if only `name` is provided, update the tag key as before.
	var tag *db.Tag
	if name != "" && nameZh == "" && nameEn == "" {
		tag, err = a.tags.Update(id, name)
	} else {
		tag, err = a.tags.Get(id)
	}
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

	if nameZh != "" {
		_, _ = a.tags.UpsertTranslation(tag.ID, service.TagTranslationInput{
			Language: "zh",
			Name:     nameZh,
		})
	}
	if nameEn != "" {
		_, _ = a.tags.UpsertTranslation(tag.ID, service.TagTranslationInput{
			Language: "en",
			Name:     nameEn,
		})
	}
	if nameZh == "" && nameEn == "" && name != "" {
		_, _ = a.tags.UpsertTranslation(tag.ID, service.TagTranslationInput{
			Language: requestLang,
			Name:     name,
		})
	}

	if refreshed, refreshErr := a.tags.Get(tag.ID); refreshErr == nil {
		tag = refreshed
	} else {
		c.Error(refreshErr)
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
