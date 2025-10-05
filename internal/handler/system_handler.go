package handler

import (
	"errors"
	"net/http"

	"github.com/commitlog/internal/service"
	"github.com/commitlog/internal/view"
	"github.com/gin-gonic/gin"
)

// HealthCheck 提供 Fly.io 与监控系统使用的健康检查端点。
func (a *API) HealthCheck(c *gin.Context) {
	sqlDB, err := a.db.DB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "database handle unavailable",
		})
		return
	}

	if err := sqlDB.PingContext(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "database unreachable",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"database": "up",
	})
}

// ShowSystemSettings 渲染系统设置页面。
func (a *API) ShowSystemSettings(c *gin.Context) {
	c.HTML(http.StatusOK, "system_settings.html", gin.H{
		"title":           "系统设置",
		"profileIconSVGs": view.ProfileIconSVGMap(),
	})
}

type systemSettingsRequest struct {
	SiteName       string `json:"siteName"`
	SiteLogoURL    string `json:"siteLogoUrl"`
	AIProvider     string `json:"aiProvider"`
	OpenAIAPIKey   string `json:"openaiApiKey"`
	DeepSeekAPIKey string `json:"deepseekApiKey"`
}

type aiTestRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
}

// GetSystemSettings 返回当前系统设置。
func (a *API) GetSystemSettings(c *gin.Context) {
	settings, err := a.system.GetSettings()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取系统设置失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"settings": systemSettingsPayload(settings)})
}

// UpdateSystemSettings 保存系统设置。
func (a *API) UpdateSystemSettings(c *gin.Context) {
	var payload systemSettingsRequest
	if !bindJSON(c, &payload, "请填写完整的系统设置") {
		return
	}

	settings, err := a.system.UpdateSettings(payload.toInput())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "保存系统设置失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "系统设置已保存",
		"settings": systemSettingsPayload(settings),
	})
}

func (r systemSettingsRequest) toInput() service.SystemSettingsInput {
	return service.SystemSettingsInput{
		SiteName:       r.SiteName,
		SiteLogoURL:    r.SiteLogoURL,
		AIProvider:     r.AIProvider,
		OpenAIAPIKey:   r.OpenAIAPIKey,
		DeepSeekAPIKey: r.DeepSeekAPIKey,
	}
}

func systemSettingsPayload(settings service.SystemSettings) gin.H {
	return gin.H{
		"siteName":       settings.SiteName,
		"siteLogoUrl":    settings.SiteLogoURL,
		"aiProvider":     settings.AIProvider,
		"openaiApiKey":   settings.OpenAIAPIKey,
		"deepseekApiKey": settings.DeepSeekAPIKey,
	}
}

// TestAIConnection 测试不同 AI 平台 API Key 的连通性。
func (a *API) TestAIConnection(c *gin.Context) {
	var payload aiTestRequest
	if !bindJSON(c, &payload, "请填写有效的 AI 配置信息") {
		return
	}

	if err := a.system.TestAIConnection(c.Request.Context(), payload.Provider, payload.APIKey); err != nil {
		switch {
		case errors.Is(err, service.ErrAIAPIKeyMissing):
			respondError(c, http.StatusBadRequest, "请填写有效的 AI API Key")
		default:
			respondError(c, http.StatusBadGateway, err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "AI 接口连接正常"})
}
