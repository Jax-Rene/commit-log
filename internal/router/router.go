package router

import (
	"html/template"

	"github.com/commitlog/internal/handler"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// SetupRouter 配置 Gin 引擎和路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 配置会话中间件
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("commitlog_session", store))

	// 加载模板并添加自定义函数
	r.SetFuncMap(template.FuncMap{
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
	})
	r.LoadHTMLGlob("web/template/admin/*.html")

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
