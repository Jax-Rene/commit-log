package handler

import (
	"net/http"

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
