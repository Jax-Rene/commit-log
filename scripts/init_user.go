package main

import (
	"fmt"
	"log"

	"github.com/commitlog/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// 初始化数据库
	if err := db.Init(); err != nil {
		log.Fatal("数据库初始化失败:", err)
	}

	// 检查是否已存在用户
	var count int64
	db.DB.Model(&db.User{}).Count(&count)
	if count > 0 {
		fmt.Println("用户已存在，无需初始化")
		return
	}

	// 创建默认管理员用户
	password := "admin123" // 默认密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("密码加密失败:", err)
	}

	user := db.User{
		Username: "admin",
		Password: string(hashedPassword),
	}

	if err := db.DB.Create(&user).Error; err != nil {
		log.Fatal("创建用户失败:", err)
	}

	fmt.Println("默认管理员用户创建成功")
	fmt.Println("用户名: admin")
	fmt.Println("密码: admin123")
}