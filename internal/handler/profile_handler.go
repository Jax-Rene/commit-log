package handler

import (
	"errors"
	"net/http"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/commitlog/internal/view"
	"github.com/gin-gonic/gin"
)

// ShowProfileContacts renders the admin page for managing profile contacts.
func (a *API) ShowProfileContacts(c *gin.Context) {
	c.HTML(http.StatusOK, "profile_contacts.html", gin.H{
		"title":              "社交联系方式",
		"profileIconOptions": view.ProfileIconOptions(),
		"profileIconSVGs":    view.ProfileIconSVGMap(),
	})
}

type profileContactRequest struct {
	Platform string `json:"platform"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	Link     string `json:"link"`
	Icon     string `json:"icon"`
	Sort     *int   `json:"sort"`
	Visible  *bool  `json:"visible"`
}

type profileContactReorderRequest struct {
	IDs []uint `json:"ids"`
}

// ListProfileContacts 返回后台管理用的联系信息列表
func (a *API) ListProfileContacts(c *gin.Context) {
	contacts, err := a.profiles.ListContacts(true)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取联系信息失败")
		return
	}

	items := make([]gin.H, 0, len(contacts))
	for _, contact := range contacts {
		items = append(items, profileContactPayload(contact))
	}

	c.JSON(http.StatusOK, gin.H{"contacts": items})
}

// CreateProfileContact 创建新的联系信息
func (a *API) CreateProfileContact(c *gin.Context) {
	var payload profileContactRequest
	if !bindJSON(c, &payload, "请填写完整的联系信息") {
		return
	}

	contact, err := a.profiles.CreateContact(payload.toInput())
	if err != nil {
		handleProfileContactError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "已新增联系信息",
		"contact": profileContactPayload(*contact),
	})
}

// UpdateProfileContact 更新联系信息
func (a *API) UpdateProfileContact(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的联系信息ID")
		return
	}

	var payload profileContactRequest
	if !bindJSON(c, &payload, "请填写完整的联系信息") {
		return
	}

	contact, err := a.profiles.UpdateContact(id, payload.toInput())
	if err != nil {
		handleProfileContactError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "联系信息已更新",
		"contact": profileContactPayload(*contact),
	})
}

// DeleteProfileContact 删除指定联系信息
func (a *API) DeleteProfileContact(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的联系信息ID")
		return
	}

	if err := a.profiles.DeleteContact(id); err != nil {
		handleProfileContactError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "联系信息已删除"})
}

// ReorderProfileContacts 更新排序
func (a *API) ReorderProfileContacts(c *gin.Context) {
	var payload profileContactReorderRequest
	if !bindJSON(c, &payload, "排序数据格式不正确") {
		return
	}

	if err := a.profiles.ReorderContacts(payload.IDs); err != nil {
		respondError(c, http.StatusInternalServerError, "更新排序失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "排序已更新"})
}

func (r profileContactRequest) toInput() service.ProfileContactInput {
	return service.ProfileContactInput{
		Platform: r.Platform,
		Label:    r.Label,
		Value:    r.Value,
		Link:     r.Link,
		Icon:     r.Icon,
		Sort:     r.Sort,
		Visible:  r.Visible,
	}
}

func profileContactPayload(contact db.ProfileContact) gin.H {
	return gin.H{
		"id":       contact.ID,
		"platform": contact.Platform,
		"label":    contact.Label,
		"value":    contact.Value,
		"link":     contact.Link,
		"icon":     contact.Icon,
		"sort":     contact.Sort,
		"visible":  contact.Visible,
	}
}

func handleProfileContactError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProfileContactNotFound):
		respondError(c, http.StatusNotFound, "联系信息不存在")
	case errors.Is(err, service.ErrProfileContactInvalidInput):
		respondError(c, http.StatusBadRequest, "请检查必填项")
	default:
		respondError(c, http.StatusInternalServerError, "操作失败")
	}
}
