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

	// Load templates
	templates := newTemplateRegistry()
	templates.LoadTemplates("web/template")
	r.HTMLRender = templates

	// 静态文件服务
	r.Static("/static", "./web/static")

	// 公共站点路由
	r.GET("/", handler.ShowHome)
	r.GET("/posts/more", handler.LoadMorePosts)
	r.GET("/posts/:id", handler.ShowPostDetail)
	r.GET("/tags", handler.ShowTagArchive)
	r.GET("/about", handler.ShowAbout)

	// 在这里定义你的路由
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// 后台管理路由
	admin := r.Group("/admin")
	{
		admin.GET("/login", handler.ShowLoginPage)
		admin.POST("/login", handler.Login)
		admin.GET("/logout", handler.Logout)

		// 需要认证的后台路由
		auth := admin.Group("")
		auth.Use(handler.AuthRequired())
		{
			auth.GET("/dashboard", handler.ShowDashboard)
			auth.GET("/posts", handler.ShowPostList)
			auth.GET("/posts/new", handler.ShowPostEdit)
			auth.GET("/posts/:id/edit", handler.ShowPostEdit)
			auth.GET("/about", handler.ShowAboutEditor)

			// API路由
			api := auth.Group("/api")
			{
				api.GET("/posts", handler.GetPosts)
				api.GET("/posts/:id", handler.GetPost)
				api.POST("/posts", handler.CreatePost)
				api.PUT("/posts/:id", handler.UpdatePost)
				api.DELETE("/posts/:id", handler.DeletePost)

				api.GET("/tags", handler.GetTags)
				api.POST("/tags", handler.CreateTag)
				api.PUT("/tags/:id", handler.UpdateTag)
				api.DELETE("/tags/:id", handler.DeleteTag)
				api.PUT("/pages/about", handler.UpdateAboutPage)

				// 图片上传接口
				api.POST("/upload/image", handler.UploadImage)
			}
		}
	}

	return r
}
