package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// API bundles shared dependencies for HTTP handlers.
type API struct {
	db              *gorm.DB
	posts           *service.PostService
	tags            *service.TagService
	pages           *service.PageService
	galleries       *service.GalleryService
	profiles        *service.ProfileService
	analytics       analyticsProvider
	system          *service.SystemSettingService
	summaries       service.SummaryGenerator
	optimizer       service.ContentOptimizer
	snippetRewriter service.SnippetRewriter
	uploadDir       string
	uploadURL       string
	baseURL         string
}

type siteViewModel struct {
	Name            string
	LogoLight       string
	LogoDark        string
	Avatar          string
	AdminFooter     string
	PublicFooter    string
	Description     string
	Keywords        string
	SocialImage     string
	GallerySubtitle string
	GalleryEnabled  bool
}

const siteSettingsContextKey = "__site_settings"

// NewAPI constructs a handler set with shared services.
func NewAPI(db *gorm.DB, uploadDir, uploadURL, baseURL string) *API {
	systemService := service.NewSystemSettingService(db)
	summaryService := service.NewAISummaryService(systemService)
	rewriteService := service.NewAIRewriteService(systemService)

	return &API{
		db:              db,
		posts:           service.NewPostService(db),
		tags:            service.NewTagService(db),
		pages:           service.NewPageService(db),
		galleries:       service.NewGalleryService(db),
		profiles:        service.NewProfileService(db),
		analytics:       service.NewAnalyticsService(db),
		system:          systemService,
		summaries:       summaryService,
		optimizer:       rewriteService,
		snippetRewriter: rewriteService,
		uploadDir:       uploadDir,
		uploadURL:       uploadURL,
		baseURL:         normalizeBaseURL(baseURL),
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
		Name:            strings.TrimSpace(settings.SiteName),
		LogoLight:       strings.TrimSpace(settings.SiteLogoURLLight),
		LogoDark:        strings.TrimSpace(settings.SiteLogoURLDark),
		AdminFooter:     strings.TrimSpace(settings.AdminFooterText),
		PublicFooter:    strings.TrimSpace(settings.PublicFooterText),
		Description:     strings.TrimSpace(settings.SiteDescription),
		Keywords:        strings.TrimSpace(settings.SiteKeywords),
		SocialImage:     strings.TrimSpace(settings.SiteSocialImage),
		GallerySubtitle: strings.TrimSpace(settings.GallerySubtitle),
		GalleryEnabled:  settings.GalleryEnabled,
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
	if view.Avatar == "" {
		view.Avatar = view.LogoLight
	}
	if view.Avatar == "" {
		view.Avatar = view.LogoDark
	}
	if view.AdminFooter == "" {
		view.AdminFooter = "日拱一卒，功不唐捐"
	}
	if view.PublicFooter == "" {
		view.PublicFooter = "激发创造，延迟满足"
	}
	if view.Description == "" {
		view.Description = "AI 全栈工程师的技术与成长记录"
	}
	if view.Keywords == "" {
		view.Keywords = service.NormalizeKeywords(view.Name)
	}

	c.Set(siteSettingsContextKey, view)
	return view
}

func (a *API) renderHTML(c *gin.Context, status int, templateName string, data gin.H) {
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

	siteDefaults := map[string]interface{}{
		"name":            view.Name,
		"logoUrl":         view.LogoLight,
		"logoUrlLight":    view.LogoLight,
		"logoUrlDark":     view.LogoDark,
		"avatar":          view.Avatar,
		"adminFooter":     view.AdminFooter,
		"publicFooter":    view.PublicFooter,
		"description":     view.Description,
		"keywords":        view.Keywords,
		"gallerySubtitle": view.GallerySubtitle,
		"galleryEnabled":  view.GalleryEnabled,
	}
	if view.SocialImage != "" {
		siteDefaults["socialImage"] = a.absoluteURL(c, view.SocialImage)
	} else {
		siteDefaults["socialImage"] = ""
	}

	if existing, ok := payload["site"]; ok {
		switch value := existing.(type) {
		case map[string]interface{}:
			for key, def := range siteDefaults {
				if _, has := value[key]; !has {
					value[key] = def
				}
			}
			payload["site"] = value
		case gin.H:
			for key, def := range siteDefaults {
				if _, has := value[key]; !has {
					value[key] = def
				}
			}
			payload["site"] = value
		default:
			payload["site"] = siteDefaults
		}
	} else {
		payload["site"] = siteDefaults
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
	if _, exists := payload["siteAvatar"]; !exists {
		payload["siteAvatar"] = view.Avatar
	}
	if _, exists := payload["siteAdminFooter"]; !exists {
		payload["siteAdminFooter"] = view.AdminFooter
	}
	if _, exists := payload["sitePublicFooter"]; !exists {
		payload["sitePublicFooter"] = view.PublicFooter
	}
	if _, exists := payload["siteDescription"]; !exists {
		payload["siteDescription"] = view.Description
	}
	if _, exists := payload["siteKeywords"]; !exists {
		payload["siteKeywords"] = view.Keywords
	}
	if _, exists := payload["siteSocialImage"]; !exists {
		if view.SocialImage != "" {
			payload["siteSocialImage"] = a.absoluteURL(c, view.SocialImage)
		} else {
			payload["siteSocialImage"] = ""
		}
	}

	seo := make(map[string]interface{})
	if existing, ok := payload["seo"]; ok {
		switch value := existing.(type) {
		case map[string]interface{}:
			for key, v := range value {
				seo[key] = v
			}
		case gin.H:
			for key, v := range value {
				seo[key] = v
			}
		}
	}

	toString := func(v interface{}) string {
		switch value := v.(type) {
		case string:
			return strings.TrimSpace(value)
		case fmt.Stringer:
			return strings.TrimSpace(value.String())
		case template.HTML:
			return strings.TrimSpace(string(value))
		case template.JS:
			return strings.TrimSpace(string(value))
		case []byte:
			return strings.TrimSpace(string(value))
		default:
			return ""
		}
	}

	canonicalSource := toString(payload["canonical"])
	canonicalURL := ""
	if canonicalSource != "" {
		canonicalURL = a.absoluteURL(c, canonicalSource)
	} else {
		canonicalURL = a.absoluteURL(c, "")
	}
	payload["canonical"] = canonicalURL

	pageTitle := toString(payload["metaTitle"])
	if pageTitle == "" {
		pageTitle = toString(payload["title"])
	}
	baseTitle := view.Name
	fullTitle := baseTitle
	if pageTitle != "" {
		fullTitle = fmt.Sprintf("%s · %s", pageTitle, baseTitle)
	}

	description := toString(payload["metaDescription"])
	if description == "" {
		description = view.Description
	}

	var keywords string
	if raw, exists := payload["metaKeywords"]; exists {
		switch value := raw.(type) {
		case []string:
			keywords = strings.Join(value, ",")
		case []interface{}:
			collected := make([]string, 0, len(value))
			for _, item := range value {
				if token := toString(item); token != "" {
					collected = append(collected, token)
				}
			}
			keywords = strings.Join(collected, ",")
		default:
			keywords = toString(value)
		}
	}
	if keywords == "" {
		keywords = view.Keywords
	}
	keywords = service.NormalizeKeywords(keywords)

	image := toString(payload["metaImage"])
	if image == "" {
		image = view.SocialImage
	}
	if image != "" {
		image = a.absoluteURL(c, image)
	}

	ogTitle := toString(payload["metaOgTitle"])
	if ogTitle == "" {
		ogTitle = pageTitle
	}
	if ogTitle == "" {
		ogTitle = baseTitle
	}

	ogType := toString(payload["metaType"])
	if ogType == "" {
		ogType = "website"
	}

	robots := toString(payload["metaRobots"])
	noindex := false
	if raw, exists := payload["noindex"]; exists {
		switch value := raw.(type) {
		case bool:
			noindex = value
		case string:
			trimmed := strings.ToLower(strings.TrimSpace(value))
			noindex = trimmed == "true" || trimmed == "1" || trimmed == "noindex"
		}
	}
	if robots == "" {
		if noindex {
			robots = "noindex,follow"
		} else {
			robots = "index,follow"
		}
	}

	formatRFC3339 := func(v interface{}) string {
		switch value := v.(type) {
		case time.Time:
			if value.IsZero() {
				return ""
			}
			return value.UTC().Format(time.RFC3339)
		case *time.Time:
			if value == nil || value.IsZero() {
				return ""
			}
			return value.UTC().Format(time.RFC3339)
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return ""
			}
			if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
				return parsed.UTC().Format(time.RFC3339)
			}
			return trimmed
		default:
			return ""
		}
	}

	published := formatRFC3339(payload["metaPublishedAt"])
	modified := formatRFC3339(payload["metaModifiedAt"])

	twitterTitle := toString(payload["metaTwitterTitle"])
	if twitterTitle == "" {
		twitterTitle = ogTitle
	}
	twitterDescription := toString(payload["metaTwitterDescription"])
	if twitterDescription == "" {
		twitterDescription = description
	}
	twitterImage := toString(payload["metaTwitterImage"])
	if twitterImage != "" {
		twitterImage = a.absoluteURL(c, twitterImage)
	} else {
		twitterImage = image
	}
	twitterCard := "summary"
	if twitterImage != "" {
		twitterCard = "summary_large_image"
	}

	toTemplateJS := func(raw interface{}) template.JS {
		switch value := raw.(type) {
		case template.JS:
			return value
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return ""
			}
			return template.JS(trimmed)
		case []byte:
			trimmed := strings.TrimSpace(string(value))
			if trimmed == "" {
				return ""
			}
			return template.JS(trimmed)
		case map[string]interface{}:
			if data, err := json.Marshal(value); err == nil {
				return template.JS(data)
			}
		case gin.H:
			if data, err := json.Marshal(value); err == nil {
				return template.JS(data)
			}
		case []interface{}:
			if data, err := json.Marshal(value); err == nil {
				return template.JS(data)
			}
		}
		return ""
	}

	structured := toTemplateJS(payload["seoJSONLD"])
	if structured == "" {
		structured = toTemplateJS(payload["metaJSONLD"])
	}

	siteJSON := ""
	if base := a.siteBaseURL(c); base != "" {
		searchTarget := strings.TrimRight(base, "/") + "/?search={search_term_string}"
		siteData := map[string]interface{}{
			"@context":        "https://schema.org",
			"@type":           "WebSite",
			"url":             base + "/",
			"name":            baseTitle,
			"description":     description,
			"potentialAction": map[string]interface{}{"@type": "SearchAction", "target": searchTarget, "query-input": "required name=search_term_string"},
		}
		if keywords != "" {
			siteData["keywords"] = keywords
		}
		if data, err := json.Marshal(siteData); err == nil {
			siteJSON = string(data)
		}
	}

	setIfMissing := func(key string, value interface{}) {
		if value == nil {
			return
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return
			}
		case template.HTML:
			if strings.TrimSpace(string(v)) == "" {
				return
			}
		case template.JS:
			if strings.TrimSpace(string(v)) == "" {
				return
			}
		}
		if _, exists := seo[key]; !exists {
			seo[key] = value
		}
	}

	setIfMissing("siteName", baseTitle)
	setIfMissing("title", pageTitle)
	setIfMissing("fullTitle", fullTitle)
	setIfMissing("description", description)
	if keywords != "" {
		setIfMissing("keywords", keywords)
	}
	setIfMissing("canonical", canonicalURL)
	setIfMissing("ogTitle", ogTitle)
	setIfMissing("ogDescription", description)
	setIfMissing("ogType", ogType)
	setIfMissing("ogURL", canonicalURL)
	if image != "" {
		setIfMissing("image", image)
		setIfMissing("ogImage", image)
	}
	setIfMissing("robots", robots)
	setIfMissing("locale", "zh_CN")
	setIfMissing("twitterCard", twitterCard)
	setIfMissing("twitterTitle", twitterTitle)
	setIfMissing("twitterDescription", twitterDescription)
	if twitterImage != "" {
		setIfMissing("twitterImage", twitterImage)
	}
	if published != "" {
		setIfMissing("published", published)
	}
	if modified != "" {
		setIfMissing("modified", modified)
	}
	if structured != "" {
		setIfMissing("jsonld", structured)
	}
	if siteJSON != "" {
		setIfMissing("siteJsonld", template.JS(siteJSON))
	}

	payload["seo"] = seo

	c.HTML(status, templateName, payload)
}

// RenderHTML 在向模板渲染时自动附加系统设置中的站点名称与 Logo 信息。
func (a *API) RenderHTML(c *gin.Context, status int, template string, data gin.H) {
	a.renderHTML(c, status, template, data)
}

func (a *API) detectScheme(c *gin.Context) string {
	candidates := []string{
		c.GetHeader("X-Forwarded-Proto"),
		c.GetHeader("X-Forwarded-Protocol"),
		c.GetHeader("X-Forwarded-Scheme"),
	}
	for _, header := range candidates {
		if header == "" {
			continue
		}
		parts := strings.Split(header, ",")
		if len(parts) > 0 {
			candidate := strings.TrimSpace(parts[0])
			if candidate != "" {
				return strings.ToLower(candidate)
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Ssl")), "on") {
		return "https"
	}
	if c.Request.TLS != nil {
		return "https"
	}
	if strings.HasPrefix(strings.ToLower(c.Request.Proto), "https") {
		return "https"
	}
	return "http"
}

func (a *API) siteBaseURL(c *gin.Context) string {
	if a.baseURL != "" {
		return a.baseURL
	}
	hostHeader := c.GetHeader("X-Forwarded-Host")
	host := strings.TrimSpace(strings.Split(hostHeader, ",")[0])
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" {
		return ""
	}
	scheme := a.detectScheme(c)
	if scheme == "" {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func (a *API) absoluteURL(c *gin.Context, path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		if c.Request.URL != nil {
			trimmed = c.Request.URL.RequestURI()
		}
	}
	if trimmed == "" {
		trimmed = "/"
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "//") {
		return a.detectScheme(c) + ":" + trimmed
	}
	base := a.siteBaseURL(c)
	if base == "" {
		return trimmed
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return base + trimmed
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	return "https://" + trimmed
}
