package handler

import (
	"strings"

	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// API bundles shared dependencies for HTTP handlers.
type API struct {
	db        *gorm.DB
	posts     *service.PostService
	tags      *service.TagService
	pages     *service.PageService
	habits    *service.HabitService
	habitLogs *service.HabitLogService
	profiles  *service.ProfileService
	analytics *service.AnalyticsService
	system    *service.SystemSettingService
	summaries service.SummaryGenerator
	uploadDir string
	uploadURL string
}

type siteViewModel struct {
	Name         string
	LogoLight    string
	LogoDark     string
	AdminFooter  string
	PublicFooter string
}

const siteSettingsContextKey = "__site_settings"

// NewAPI constructs a handler set with shared services.
func NewAPI(db *gorm.DB, uploadDir, uploadURL string) *API {
	systemService := service.NewSystemSettingService(db)
	summaryService := service.NewAISummaryService(systemService)

	return &API{
		db:        db,
		posts:     service.NewPostService(db),
		tags:      service.NewTagService(db),
		pages:     service.NewPageService(db),
		habits:    service.NewHabitService(db),
		habitLogs: service.NewHabitLogService(db),
		profiles:  service.NewProfileService(db),
		analytics: service.NewAnalyticsService(db),
		system:    systemService,
		summaries: summaryService,
		uploadDir: uploadDir,
		uploadURL: uploadURL,
	}
}

// DB exposes the underlying gorm instance for legacy paths.
func (a *API) DB() *gorm.DB {
	return a.db
}

func (a *API) siteSettings(c *gin.Context) siteViewModel {
	if cached, exists := c.Get(siteSettingsContextKey); exists {
		if view, ok := cached.(siteViewModel); ok {
			return view
		}
	}

	settings, err := a.system.GetSettings()
	if err != nil {
		c.Error(err)
	}

	view := siteViewModel{
		Name:         strings.TrimSpace(settings.SiteName),
		LogoLight:    strings.TrimSpace(settings.SiteLogoURLLight),
		LogoDark:     strings.TrimSpace(settings.SiteLogoURLDark),
		AdminFooter:  strings.TrimSpace(settings.AdminFooterText),
		PublicFooter: strings.TrimSpace(settings.PublicFooterText),
	}
	if view.Name == "" {
		view.Name = "CommitLog"
	}
	if view.LogoLight == "" {
		fallback := strings.TrimSpace(settings.SiteLogoURL)
		if fallback == "" {
			fallback = view.LogoDark
		}
		view.LogoLight = fallback
	}
	if view.LogoDark == "" {
		view.LogoDark = view.LogoLight
	}
	if view.AdminFooter == "" {
		view.AdminFooter = "日拱一卒，功不唐捐"
	}
	if view.PublicFooter == "" {
		view.PublicFooter = "激发创造，延迟满足"
	}

	c.Set(siteSettingsContextKey, view)
	return view
}

func (a *API) renderHTML(c *gin.Context, status int, template string, data gin.H) {
	view := a.siteSettings(c)

	var payload gin.H
	if data == nil {
		payload = gin.H{}
	} else {
		payload = gin.H{}
		for key, value := range data {
			payload[key] = value
		}
	}

	if _, exists := payload["site"]; !exists {
		payload["site"] = gin.H{
			"name":         view.Name,
			"logoUrl":      view.LogoLight,
			"logoUrlLight": view.LogoLight,
			"logoUrlDark":  view.LogoDark,
			"adminFooter":  view.AdminFooter,
			"publicFooter": view.PublicFooter,
		}
	}
	if _, exists := payload["siteName"]; !exists {
		payload["siteName"] = view.Name
	}
	if _, exists := payload["siteLogoUrl"]; !exists {
		payload["siteLogoUrl"] = view.LogoLight
	}
	if _, exists := payload["siteLogoUrlLight"]; !exists {
		payload["siteLogoUrlLight"] = view.LogoLight
	}
	if _, exists := payload["siteLogoUrlDark"]; !exists {
		payload["siteLogoUrlDark"] = view.LogoDark
	}
	if _, exists := payload["siteAdminFooter"]; !exists {
		payload["siteAdminFooter"] = view.AdminFooter
	}
	if _, exists := payload["sitePublicFooter"]; !exists {
		payload["sitePublicFooter"] = view.PublicFooter
	}

	c.HTML(status, template, payload)
}

// RenderHTML 在向模板渲染时自动附加系统设置中的站点名称与 Logo 信息。
func (a *API) RenderHTML(c *gin.Context, status int, template string, data gin.H) {
	a.renderHTML(c, status, template, data)
}
