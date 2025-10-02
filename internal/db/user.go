package db

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User 定义了用户模型
type User struct {
	gorm.Model
	Username string `gorm:"unique;not null"`
	Password string `gorm:"not null"`
}

// EnsureUser 存在性检查：若提供的用户名与密码均非空且不存在对应账号，则创建一个 bcrypt 哈希的用户。
func EnsureUser(username, password string) error {
	trimmedUser := strings.TrimSpace(username)
	trimmedPassword := strings.TrimSpace(password)
	if trimmedUser == "" || trimmedPassword == "" {
		return nil
	}

	if DB == nil {
		return errors.New("database not initialized")
	}

	var existing User
	if err := DB.Where("username = ?", trimmedUser).First(&existing).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(trimmedPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		return DB.Create(&User{Username: trimmedUser, Password: string(hashed)}).Error
	}

	return nil
}
