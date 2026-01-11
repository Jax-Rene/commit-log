package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/handler"
	"github.com/commitlog/internal/locale"
	"github.com/commitlog/internal/view"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

// templateRegistry holds the parsed templates.
type templateRegistry struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

type errorAction struct {
	Label string
	Href  string
}

var imagePattern = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)

// newTemplateRegistry creates a new template registry.
func newTemplateRegistry() *templateRegistry {
	return &templateRegistry{
		templates: make(map[string]*template.Template),
		funcMap: template.FuncMap{
			"add": func(a, b int) int {
				return a + b
			},
			"sub": func(a, b int) int {
				return a - b
			},
			"mul": func(a, b int) int {
				return a * b
			},
			"gt": func(a, b int) bool {
				return a > b
			},
			"lt": func(a, b int) bool {
				return a < b
			},
			"eq": func(a, b interface{}) bool {
				return a == b
			},
			"dict": func(values ...interface{}) (map[string]interface{}, error) {
				if len(values)%2 != 0 {
					return nil, errors.New("invalid dict call")
				}
				dict := make(map[string]interface{}, len(values)/2)
				for i := 0; i < len(values); i += 2 {
					key, ok := values[i].(string)
					if !ok {
						return nil, errors.New("dict keys must be strings")
					}
					dict[key] = values[i+1]
				}
				return dict, nil
			},
			"formatDate": func(t time.Time) string {
				if t.IsZero() {
					return ""
				}
				return t.Format("2006-01-02")
			},
			"firstImage": func(content string) string {
				match := imagePattern.FindStringSubmatch(content)
				if len(match) >= 2 {
					return match[1]
				}
				return ""
			},
			"initials": func(title string) string {
				title = strings.TrimSpace(title)
				if title == "" {
					return "CL"
				}
				runes := []rune(title)
				if len(runes) == 1 {
					return strings.ToUpper(string(runes[0]))
				}
				return strings.ToUpper(string(runes[0:2]))
			},
			"profileIcon": func(key string) template.HTML {
				return template.HTML(view.ProfileIconSVG(key))
			},
			"profileContactTitle": func(label, value string) string {
				trimmedLabel := strings.TrimSpace(label)
				trimmedValue := strings.TrimSpace(value)
				if trimmedLabel == "" {
					return trimmedValue
				}
				if trimmedValue == "" {
					return trimmedLabel
				}
				return fmt.Sprintf("%s：%s", trimmedLabel, trimmedValue)
			},
			"toJSON": func(v interface{}) template.JS {
				data, err := json.Marshal(v)
				if err != nil {
					return template.JS("null")
				}
				return template.JS(data)
			},
			"tr": func(language, english, chinese string) string {
				return locale.Pick(language, english, chinese)
			},
			"accent": func(text string) string {
				palette := []string{
					"from-sky-400 via-blue-500 to-indigo-500",
					"from-emerald-400 via-teal-500 to-blue-500",
					"from-rose-400 via-pink-500 to-fuchsia-500",
					"from-amber-300 via-orange-400 to-rose-400",
					"from-purple-400 via-indigo-500 to-blue-500",
				}
				sum := 0
				for _, r := range text {
					sum += int(r)
				}
				idx := sum % len(palette)
				return palette[idx]
			},
			"aspectPadding": func(width, height int) string {
				if width <= 0 || height <= 0 {
					return "66.67%"
				}
				ratio := float64(height) / float64(width) * 100
				return fmt.Sprintf("%.2f%%", ratio)
			},
			"truncate": func(text string, length int) string {
				runes := []rune(strings.TrimSpace(text))
				if length <= 0 || len(runes) <= length {
					return strings.TrimSpace(text)
				}
				return strings.TrimSpace(string(runes[:length])) + "…"
			},
		},
	}
}

// LoadTemplates loads all templates from the given path.
func (r *templateRegistry) LoadTemplates(path string) {
	root := resolveTemplateRoot(path)

	componentTemplates, err := filepath.Glob(filepath.Join(root, "components", "*.html"))
	if err != nil {
		panic(err)
	}

	adminBase := filepath.Join(root, "layout", "admin_base.html")
	authBase := filepath.Join(root, "layout", "auth_base.html")
	publicBase := filepath.Join(root, "layout", "public_base.html")

	adminPages, err := filepath.Glob(filepath.Join(root, "admin", "*.html"))
	if err != nil {
		panic(err)
	}
	publicPages, err := filepath.Glob(filepath.Join(root, "public", "*.html"))
	if err != nil {
		panic(err)
	}
	partialTemplates, err := filepath.Glob(filepath.Join(root, "public", "partials", "*.html"))
	if err != nil {
		panic(err)
	}

	build := func(pages []string, base string, overrides map[string]string) {
		for _, page := range pages {
			templateName := filepath.Base(page)
			baseFile := base
			if overrides != nil {
				if override, ok := overrides[templateName]; ok && override != "" {
					baseFile = override
				}
			}
			files := append([]string{baseFile}, componentTemplates...)
			files = append(files, page)
			files = append(files, partialTemplates...)
			tmpl := template.New(templateName).Funcs(r.funcMap)
			r.templates[templateName] = template.Must(tmpl.ParseFiles(files...))
		}
	}

	build(adminPages, adminBase, map[string]string{
		"login.html":       authBase,
		"login_error.html": authBase,
	})
	build(publicPages, publicBase, nil)

	for _, partial := range partialTemplates {
		templateName := filepath.Base(partial)
		files := append([]string{}, componentTemplates...)
		files = append(files, partial)
		tmpl := template.New(templateName).Funcs(r.funcMap)
		r.templates[templateName] = template.Must(tmpl.ParseFiles(files...))
	}
}

func resolveTemplateRoot(path string) string {
	candidates := []string{
		path,
		filepath.Join("..", path),
		filepath.Join("..", "..", path),
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate
		}
	}
	return path
}

// Instance returns a render.Render instance for the given template name.
func (r *templateRegistry) Instance(name string, data interface{}) render.Render {
	tmpl := r.templates[name]
	execName := name
	if tmpl.Lookup("base") != nil {
		execName = "base"
	}

	return render.HTML{
		Template: tmpl,
		Name:     execName,
		Data:     data,
	}
}

// SetupRouter 配置 Gin 引擎和路由
func SetupRouter(sessionSecret, uploadDir, uploadURLPath, siteBaseURL string) *gin.Engine {
	r := gin.New()

	handlers := handler.NewAPI(db.DB, uploadDir, uploadURLPath, siteBaseURL)

	r.Use(gin.Logger())
	r.Use(recoveryWithHandler(handlers))
	r.Use(handlers.LocaleMiddleware())

	// 配置会话中间件
	trimmedSecret := strings.TrimSpace(sessionSecret)
	if trimmedSecret == "" {
		trimmedSecret = "commitlog-dev-secret"
	}
	store := cookie.NewStore([]byte(trimmedSecret))
	r.Use(sessions.Sessions("commitlog_session", store))

	// Load templates
	templates := newTemplateRegistry()
	templates.LoadTemplates("web/template")
	r.HTMLRender = templates

	// 静态文件服务
	r.Static("/static", "./web/static")

	trimmedUploadPath := strings.TrimSpace(uploadURLPath)
	if trimmedUploadPath == "" {
		trimmedUploadPath = "/uploads"
	}
	if !strings.HasPrefix(trimmedUploadPath, "/") {
		trimmedUploadPath = "/" + trimmedUploadPath
	}
	if strings.TrimSpace(uploadDir) != "" {
		if !strings.HasPrefix(trimmedUploadPath, "/static") {
			r.Static(trimmedUploadPath, uploadDir)
		}
		if trimmedUploadPath != "/uploads" { // 兼容旧版固定路径
			r.Static("/uploads", uploadDir)
		}
	}

	r.GET("/robots.txt", handlers.ShowRobots)
	r.GET("/sitemap.xml", handlers.ShowSitemap)
	r.GET("/rss.xml", handlers.ShowRSS)
	// 公共站点路由
	r.GET("/", handlers.ShowHome)
	r.GET("/search/suggestions", handlers.SearchSuggestions)
	r.GET("/posts/more", handlers.LoadMorePosts)
	r.GET("/posts/:id", handlers.ShowPostDetail)
	r.GET("/tags", handlers.ShowTagArchive)
	r.GET("/about", handlers.ShowAbout)
	r.GET("/gallery", handlers.ShowGallery)
	r.GET("/gallery/more", handlers.LoadMoreGallery)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.GET("/healthz", handlers.HealthCheck)

	// 后台管理路由
	admin := r.Group("/admin")
	{
		admin.GET("/login", handlers.ShowLoginPage)
		admin.POST("/login", handlers.Login)
		admin.GET("/logout", handlers.Logout)

		// 需要认证的后台路由
		auth := admin.Group("")
		auth.Use(handlers.AuthRequired())
		{
			auth.GET("/dashboard", handlers.ShowDashboard)
			auth.GET("/posts", handlers.ShowPostList)
			auth.GET("/posts/new", handlers.ShowPostEdit)
			auth.GET("/posts/:id/edit", handlers.ShowPostEdit)
			auth.GET("/gallery", handlers.ShowGalleryManagement)
			auth.GET("/tags", handlers.ShowTagManagement)
			auth.GET("/about", handlers.ShowAboutEditor)
			auth.GET("/profile/contacts", handlers.ShowProfileContacts)
			auth.GET("/system/settings", handlers.ShowSystemSettings)

			// API路由
			api := auth.Group("/api")
			{
				api.GET("/posts", handlers.GetPosts)
				api.GET("/posts/:id", handlers.GetPost)
				api.POST("/posts", handlers.CreatePost)
				api.POST("/posts/summary", handlers.GeneratePostSummary)
				api.POST("/posts/optimize", handlers.OptimizePostContent)
				api.POST("/posts/chat", handlers.RewritePostSelection)
				api.POST("/posts/:id/publish", handlers.PublishPost)
				api.PUT("/posts/:id", handlers.UpdatePost)
				api.DELETE("/posts/:id", handlers.DeletePost)

				api.GET("/gallery", handlers.ListGalleryImages)
				api.POST("/gallery", handlers.CreateGalleryImage)
				api.PUT("/gallery/:id", handlers.UpdateGalleryImage)
				api.DELETE("/gallery/:id", handlers.DeleteGalleryImage)

				api.GET("/tags", handlers.GetTags)
				api.POST("/tags", handlers.CreateTag)
				api.PUT("/tags/:id", handlers.UpdateTag)
				api.DELETE("/tags/:id", handlers.DeleteTag)
				api.PUT("/pages/about", handlers.UpdateAboutPage)
				api.GET("/profile/contacts", handlers.ListProfileContacts)
				api.POST("/profile/contacts", handlers.CreateProfileContact)
				api.PUT("/profile/contacts/:id", handlers.UpdateProfileContact)
				api.DELETE("/profile/contacts/:id", handlers.DeleteProfileContact)
				api.PUT("/profile/contacts/order", handlers.ReorderProfileContacts)
				api.GET("/system/settings", handlers.GetSystemSettings)
				api.PUT("/system/settings", handlers.UpdateSystemSettings)
				api.POST("/system/settings/ai/test", handlers.TestAIConnection)

				// 图片上传接口
				api.POST("/upload/image", handlers.UploadImage)
			}
		}
	}

	r.NoRoute(func(c *gin.Context) {
		if prefersJSON(c) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "资源不存在"})
			return
		}

		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/admin") {
			renderErrorPage(c, handlers, http.StatusNotFound,
				pickLocaleText(c, handlers, "Admin page not found", "后台页面走丢了"),
				pickLocaleText(c, handlers, "This link may have moved or access changed. Return to the dashboard.", "该链接可能被移动或权限已变更，返回仪表盘继续管理站点。"),
				&errorAction{Label: pickLocaleText(c, handlers, "Back to dashboard", "返回仪表盘"), Href: "/admin/dashboard"},
				&errorAction{Label: pickLocaleText(c, handlers, "Back home", "回到首页"), Href: "/"},
			)
			return
		}

		renderErrorPage(c, handlers, http.StatusNotFound,
			pickLocaleText(c, handlers, "Page not found", "页面走丢了"),
			pickLocaleText(c, handlers, "We couldn't find that content. Try the homepage or browse tags.", "我们没有找到你想访问的内容，试试回到首页或浏览其他栏目。"),
			&errorAction{Label: pickLocaleText(c, handlers, "Back home", "返回首页"), Href: "/"},
			&errorAction{Label: pickLocaleText(c, handlers, "Browse tags", "查看全部标签"), Href: "/tags"},
		)
	})

	return r
}

func recoveryWithHandler(handlers *handler.API) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(gin.DefaultErrorWriter, func(c *gin.Context, recovered interface{}) {
		if recovered != nil {
			fmt.Fprintf(gin.DefaultErrorWriter, "panic recovered: %v\n", recovered)
		}

		if prefersJSON(c) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "服务器开小差了，请稍后再试"})
			return
		}

		path := c.Request.URL.Path
		var primary *errorAction
		var secondary *errorAction
		if strings.HasPrefix(path, "/admin") {
			primary = &errorAction{Label: pickLocaleText(c, handlers, "Back to dashboard", "返回仪表盘"), Href: "/admin/dashboard"}
			secondary = &errorAction{Label: pickLocaleText(c, handlers, "Back home", "回到首页"), Href: "/"}
		} else {
			primary = &errorAction{Label: pickLocaleText(c, handlers, "Back home", "返回首页"), Href: "/"}
			secondary = &errorAction{Label: pickLocaleText(c, handlers, "Contact the author", "联系站长"), Href: "/about"}
		}

		renderErrorPage(c, handlers, http.StatusInternalServerError,
			pickLocaleText(c, handlers, "Server error", "服务器开小差了"),
			pickLocaleText(c, handlers, "We've logged the issue. Please try again later.", "我们已经记录了这个问题，请稍后再试。"),
			primary,
			secondary,
		)
		c.Abort()
	})
}

func renderErrorPage(c *gin.Context, handlers *handler.API, status int, headline, description string, primary, secondary *errorAction) {
	handlers.RenderHTML(c, status, "error.html", gin.H{
		"title":           fmt.Sprintf("%d %s", status, http.StatusText(status)),
		"status":          status,
		"statusText":      http.StatusText(status),
		"headline":        headline,
		"description":     description,
		"primaryAction":   primary,
		"secondaryAction": secondary,
		"year":            time.Now().Year(),
	})
}

func pickLocaleText(c *gin.Context, handlers *handler.API, english, chinese string) string {
	if handlers == nil {
		return locale.Pick("", english, chinese)
	}
	return locale.Pick(handlers.RequestLanguage(c), english, chinese)
}

func prefersJSON(c *gin.Context) bool {
	accept := strings.ToLower(c.GetHeader("Accept"))
	if strings.Contains(accept, "application/json") || strings.Contains(accept, "application/problem+json") {
		return true
	}

	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/admin/api") {
		return true
	}

	contentType := strings.ToLower(c.ContentType())
	return strings.Contains(contentType, "application/json")
}
