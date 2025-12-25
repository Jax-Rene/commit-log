package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ShowLoginPage 渲染登录页面
func (a *API) ShowLoginPage(c *gin.Context) {
	a.renderHTML(c, http.StatusOK, "login.html", gin.H{
		"title": "管理员登录",
	})
}

// Login 处理用户登录请求 - 简化版，假设所有请求都来自HTMX
func (a *API) Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	remember := c.PostForm("remember") == "1"

	// 查找用户
	var user db.User
	if err := a.db.Where("username = ?", username).First(&user).Error; err != nil {
		a.renderHTML(c, http.StatusUnauthorized, "login_error.html", gin.H{"error": "用户名或密码错误"})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		a.renderHTML(c, http.StatusUnauthorized, "login_error.html", gin.H{"error": "用户名或密码错误"})
		return
	}

	// 设置会话
	session := sessions.Default(c)
	options := sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
	}

	if remember {
		options.MaxAge = 30 * 24 * 60 * 60
	} else {
		options.MaxAge = 0
	}

	session.Options(options)
	session.Set("user_id", user.ID)
	session.Set("username", user.Username)
	if err := session.Save(); err != nil {
		a.renderHTML(c, http.StatusInternalServerError, "login_error.html", gin.H{"error": "会话保存失败"})
		return
	}

	// HTMX重定向
	c.Redirect(http.StatusFound, "/admin/dashboard")
}

// Logout 处理用户登出
func (a *API) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/admin/login")
}

// ShowDashboard 渲染后台主面板
func (a *API) ShowDashboard(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")

	trendHours := 24
	trendRangeType := "hours"
	trendRangeValue := 24
	trendRangeLabel := "24 小时"
	if daysParam := c.Query("days"); daysParam != "" {
		if days, err := strconv.Atoi(daysParam); err == nil {
			switch days {
			case 1, 3, 7, 14, 30:
				trendHours = days * 24
				trendRangeType = "days"
				trendRangeValue = days
				trendRangeLabel = strconv.Itoa(days) + " 天"
			}
		}
	} else if hoursParam := c.Query("hours"); hoursParam != "" {
		if hours, err := strconv.Atoi(hoursParam); err == nil {
			switch hours {
			case 6, 12, 24, 48, 72:
				trendHours = hours
				trendRangeType = "hours"
				trendRangeValue = hours
				trendRangeLabel = strconv.Itoa(hours) + " 小时"
			}
		}
	}

	// 获取文章总数
	var postCount int64
	a.db.Model(&db.Post{}).Count(&postCount)

	// 获取标签总数
	var tagCount int64
	a.db.Model(&db.Tag{}).Count(&tagCount)

	var overview service.SiteOverview
	var hourlyTrend []service.HourlyTrafficPoint
	if a.analytics != nil {
		if ov, err := a.analytics.Overview(5); err == nil {
			overview = ov
		} else {
			c.Error(err)
		}
		if trend, err := a.analytics.HourlyTrafficTrend(time.Now().UTC(), trendHours); err == nil {
			hourlyTrend = trend
		} else {
			c.Error(err)
		}
	}

	a.renderHTML(c, http.StatusOK, "dashboard.html", gin.H{
		"title":           "管理面板",
		"username":        username,
		"postCount":       postCount,
		"tagCount":        tagCount,
		"overview":        overview,
		"trend":           hourlyTrend,
		"trendHours":      trendHours,
		"trendRangeType":  trendRangeType,
		"trendRangeValue": trendRangeValue,
		"trendRangeLabel": trendRangeLabel,
	})
}

// AuthRequired 是一个简单的认证中间件
func (a *API) AuthRequired() gin.HandlerFunc {
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
