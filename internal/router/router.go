package router

import (
	"errors"
	"html/template"
	"path/filepath"

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
		},
	}
}

// LoadTemplates loads all templates from the given path.
func (r *templateRegistry) LoadTemplates(path string) {
	baseTemplates, err := filepath.Glob(filepath.Join(path, "layout", "*.html"))
	if err != nil {
		panic(err)
	}
	componentTemplates, err := filepath.Glob(filepath.Join(path, "components", "*.html"))
	if err != nil {
		panic(err)
	}

	pageTemplates, err := filepath.Glob(filepath.Join(path, "admin", "*.html"))
	if err != nil {
		panic(err)
	}

	for _, page := range pageTemplates {
		templateName := filepath.Base(page)

		files := []string{}
		files = append(files, baseTemplates...)
		files = append(files, componentTemplates...)
		files = append(files, page)

		tmpl := template.New(templateName).Funcs(r.funcMap)
		r.templates[templateName] = template.Must(tmpl.ParseFiles(files...))
	}
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
			auth.GET("/tags", handler.ShowTagList)

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
			}
		}
	}

	return r
}
