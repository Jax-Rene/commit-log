package router

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/handler"
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

var imagePattern = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)

var profileIconMap = map[string]string{
	"wechat":   `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20.25 8.511C21.134 8.795 21.75 9.639 21.75 10.608v4.286c0 1.136-.847 2.1-1.98 2.192-.339.027-.678.052-1.02.072V20.25L15.75 17.25c-1.354 0-2.695-.055-4.02-.164-.298-.024-.577-.11-.825-.242M20.25 8.511a2.4 2.4 0 0 0-.476-.095C18.447 8.306 17.105 8.25 15.75 8.25c-1.355 0-2.697.056-4.024.166-1.131.093-1.976 1.056-1.976 2.192v4.285c0 .838.46 1.582 1.155 1.952M20.25 8.511V6.637c0-1.621-1.152-3.027-2.76-3.235A53.77 53.77 0 0 0 11.25 3c-2.115 0-4.198.137-6.24.402C3.402 3.61 2.25 5.016 2.25 6.637v6.225c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.139 1.74.193V21l4.155-4.155"/></svg>`,
	"github":   `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.142-1.335-1.512-1.335-1.512-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>`,
	"email":    `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15A2.25 2.25 0 0 1 2.25 17.25V6.75M21.75 6.75A2.25 2.25 0 0 0 19.5 4.5h-15A2.25 2.25 0 0 0 2.25 6.75v.243c0 .781.405 1.506 1.071 1.916l7.5 4.615a2.25 2.25 0 0 0 2.157 0l7.5-4.615a2.25 2.25 0 0 0 1.072-1.916V6.75"/></svg>`,
	"telegram": `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z"/></svg>`,
	"x":        `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M18.901 1.153h3.68l-8.04 9.19L24 22.846h-7.406l-5.8-7.584-6.638 7.584H.474l8.6-9.83L0 1.154h7.594l5.243 6.932ZM17.61 20.644h2.039L6.486 3.24H4.298Z"/></svg>`,
	"website":  `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21c4.193 0 7.716-2.867 8.716-6.747M12 21c-4.193 0-7.716-2.867-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9s-2.015-9-4.5-9m0 18c-2.485 0-4.5-4.03-4.5-9s2.015-9 4.5-9m0-0c3.365 0 6.299 1.847 7.843 4.582M12 3c-3.365 0-6.299 1.847-7.843 4.582m15.686 0c.737 1.305 1.157 2.812 1.157 4.418 0 .778-.099 1.533-.284 2.253m-.873 4.836C18.133 15.685 15.162 16.5 12 16.5s-6.134-.815-8.716-2.247m0 0A8.948 8.948 0 0 1 3 12c0-1.605.42-3.112 1.157-4.417"/></svg>`,
	"default":  `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17.982 18.725C16.612 16.918 14.442 15.75 12 15.75s-4.612 1.168-5.982 2.975M17.982 18.725A8.97 8.97 0 0 0 21 12c0-4.971-4.03-9-9-9s-9 4.029-9 9a8.97 8.97 0 0 0 3.018 6.725M17.982 18.725C16.392 20.14 14.296 21 12 21s-4.392-.86-5.982-2.275M15 9.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/></svg>`,
}

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
				trimmed := strings.ToLower(strings.TrimSpace(key))
				if svg, ok := profileIconMap[trimmed]; ok && svg != "" {
					return template.HTML(svg)
				}
				return template.HTML(profileIconMap["default"])
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
			"habitFrequencyText": func(unit string, count int) string {
				switch strings.ToLower(strings.TrimSpace(unit)) {
				case "daily":
					return fmt.Sprintf("每天 %d 次", count)
				case "weekly":
					return fmt.Sprintf("每周 %d 次", count)
				case "monthly":
					return fmt.Sprintf("每月 %d 次", count)
				default:
					return fmt.Sprintf("%s %d 次", unit, count)
				}
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
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 配置会话中间件
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("commitlog_session", store))

	handlers := handler.NewAPI(db.DB)

	// Load templates
	templates := newTemplateRegistry()
	templates.LoadTemplates("web/template")
	r.HTMLRender = templates

	// 静态文件服务
	r.Static("/static", "./web/static")

	// 公共站点路由
	r.GET("/", handlers.ShowHome)
	r.GET("/posts/more", handlers.LoadMorePosts)
	r.GET("/posts/:id", handlers.ShowPostDetail)
	r.GET("/tags", handlers.ShowTagArchive)
	r.GET("/about", handlers.ShowAbout)

	// 在这里定义你的路由
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

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
			auth.GET("/habits", handlers.ShowHabitList)
			auth.GET("/habits/new", handlers.ShowHabitEdit)
			auth.GET("/habits/:id/edit", handlers.ShowHabitEdit)
			auth.GET("/posts", handlers.ShowPostList)
			auth.GET("/posts/new", handlers.ShowPostEdit)
			auth.GET("/posts/:id/edit", handlers.ShowPostEdit)
			auth.GET("/about", handlers.ShowAboutEditor)
			auth.GET("/profile/contacts", handlers.ShowProfileContacts)

			// API路由
			api := auth.Group("/api")
			{
				api.GET("/posts", handlers.GetPosts)
				api.GET("/posts/:id", handlers.GetPost)
				api.POST("/posts", handlers.CreatePost)
				api.PUT("/posts/:id", handlers.UpdatePost)
				api.DELETE("/posts/:id", handlers.DeletePost)

				api.GET("/habits", handlers.ListHabits)
				api.GET("/habits/heatmap", handlers.GetHabitHeatmap)
				api.GET("/habits/:id", handlers.GetHabit)
				api.POST("/habits", handlers.CreateHabit)
				api.PUT("/habits/:id", handlers.UpdateHabit)
				api.DELETE("/habits/:id", handlers.DeleteHabit)
				api.GET("/habits/:id/calendar", handlers.GetHabitCalendar)
				api.POST("/habits/:id/logs", handlers.QuickLogHabit)
				api.DELETE("/habits/:id/logs/:logId", handlers.DeleteHabitLog)

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

				// 图片上传接口
				api.POST("/upload/image", handlers.UploadImage)
			}
		}
	}

	return r
}
