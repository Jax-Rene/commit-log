package main

import (
	"log"

	"github.com/commitlog/internal/config"
	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/router"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	gin.SetMode(cfg.GinMode)

	// 初始化数据库
	if err := db.Init(cfg.DatabasePath); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

	// 设置并运行 Gin 服务器
	r := router.SetupRouter(cfg.SessionSecret)
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
