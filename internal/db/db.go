package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DB 是一个全局的数据库连接实例
var DB *gorm.DB

// Init 初始化数据库连接并执行自动迁移
func Init() error {
	var err error
	// 连接到 SQLite 数据库。如果文件不存在，GORM 会自动创建它。
	DB, err = gorm.Open(sqlite.Open("commitlog.db"), &gorm.Config{})
	if err != nil {
		return err
	}

	// 自动迁移模式，为核心模型创建表
	err = DB.AutoMigrate(&User{}, &Post{}, &Tag{}, &Page{})
	if err != nil {
		return err
	}

	return nil
}
