package main

import (
	"log"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/router"
)

func main() {
	// 初始化数据库
	if err := db.Init(); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

	// 设置并运行 Gin 服务器
	r := router.SetupRouter()
	if err := r.Run(":8082"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
