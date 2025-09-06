package router

import "github.com/gin-gonic/gin"

// SetupRouter 配置 Gin 引擎和路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 在这里定义你的路由
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	return r
}
