package config

import (
	"fmt"
	"os"
	"strings"
)

// AppConfig 汇总运行服务所需的基础配置。
type AppConfig struct {
	ListenAddr    string
	Port          string
	DatabasePath  string
	SessionSecret string
	GinMode       string
}

// Load 从环境变量读取应用配置，并为缺失项提供安全的默认值。
func Load() AppConfig {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	listenAddr := strings.TrimSpace(os.Getenv("LISTEN_ADDR"))
	if listenAddr == "" {
		listenAddr = fmt.Sprintf(":%s", port)
	}

	databasePath := strings.TrimSpace(os.Getenv("DATABASE_PATH"))
	if databasePath == "" {
		databasePath = "commitlog.db"
	}

	sessionSecret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if sessionSecret == "" {
		sessionSecret = "commitlog-dev-secret"
	}

	ginMode := strings.TrimSpace(os.Getenv("GIN_MODE"))
	if ginMode == "" {
		ginMode = "release"
	}

	return AppConfig{
		ListenAddr:    listenAddr,
		Port:          port,
		DatabasePath:  databasePath,
		SessionSecret: sessionSecret,
		GinMode:       ginMode,
	}
}
