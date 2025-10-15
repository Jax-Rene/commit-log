package db

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DB 是一个全局的数据库连接实例
var DB *gorm.DB

// Init 初始化数据库连接并执行自动迁移。
// databasePath 为空时将回退到默认值 commitlog.db。
func Init(databasePath string) error {
	path := strings.TrimSpace(databasePath)
	if path == "" {
		path = "commitlog.db"
	}

	if err := ensureParentDir(path); err != nil {
		return err
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return err
	}

	// 自动迁移模式，为核心模型创建表
	if err = DB.AutoMigrate(&User{}, &Post{}, &PostPublication{}, &Tag{}, &Page{}, &ProfileContact{}, &PostStatistic{}, &PostVisit{}, &SystemSetting{}); err != nil {
		return err
	}

	return nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}

	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return errors.New("database path parent is not a directory")
		}
		return nil
	}

	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}

	return err
}
