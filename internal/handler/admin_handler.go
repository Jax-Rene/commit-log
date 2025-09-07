package handler

import (
	"net/http"

	"github.com/commitlog/internal/db"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ShowLoginPage 渲染登录页面
func ShowLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "管理员登录",
	})
}

// Login 处理用户登录请求 - 简化版，假设所有请求都来自HTMX
func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	// 查找用户
	var user db.User
	if err := db.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.HTML(http.StatusUnauthorized, "login_error.html", gin.H{"error": "用户名或密码错误"})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		c.HTML(http.StatusUnauthorized, "login_error.html", gin.H{"error": "用户名或密码错误"})
		return
	}

	// 设置会话
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("username", user.Username)
	if err := session.Save(); err != nil {
		c.HTML(http.StatusInternalServerError, "login_error.html", gin.H{"error": "会话保存失败"})
		return
	}

	// HTMX重定向
	c.Redirect(http.StatusFound, "/admin/dashboard")
}

// Logout 处理用户登出
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/admin/login")
}

// ShowDashboard 渲染后台主面板
func ShowDashboard(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")

	// 获取文章总数
	var postCount int64
	db.DB.Model(&db.Post{}).Count(&postCount)

	// 获取标签总数
	var tagCount int64
	db.DB.Model(&db.Tag{}).Count(&tagCount)

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":     "管理面板",
		"username":  username,
		"postCount": postCount,
		"tagCount":  tagCount,
	})
}

// AuthRequired 是一个简单的认证中间件
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}
		c.Next()
	}
}
