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
	if err = DB.AutoMigrate(
		&User{},
		&Post{},
		&PostPublication{},
		&Tag{},
		&Page{},
		&GalleryImage{},
		&ProfileContact{},
		&PostStatistic{},
		&PostVisit{},
		&SiteHourlySnapshot{},
		&SiteHourlyVisitor{},
		&SystemSetting{},
	); err != nil {
		return err
	}

	migrator := DB.Migrator()
	if migrator.HasColumn(&Post{}, "title") {
		if dropErr := migrator.DropColumn(&Post{}, "title"); dropErr != nil {
			return dropErr
		}
	}

	if migrator.HasColumn(&PostPublication{}, "title") {
		if dropErr := migrator.DropColumn(&PostPublication{}, "title"); dropErr != nil {
			return dropErr
		}
	}

	if migrator.HasIndex(&Page{}, "idx_pages_slug") {
		if dropErr := migrator.DropIndex(&Page{}, "idx_pages_slug"); dropErr != nil {
			return dropErr
		}
	}

	if err := DB.Model(&Post{}).
		Where("language = '' OR language IS NULL").
		Update("language", "zh").Error; err != nil {
		return err
	}
	if err := DB.Model(&Post{}).
		Where("translation_group_id IS NULL OR translation_group_id = 0").
		Update("translation_group_id", gorm.Expr("id")).Error; err != nil {
		return err
	}
	if err := DB.Model(&Page{}).
		Where("language = '' OR language IS NULL").
		Update("language", "zh").Error; err != nil {
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
