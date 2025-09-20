package main

import (
	"fmt"
	"log"

	"github.com/commitlog/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// 测试数据生成器
func main() {
	// 初始化数据库
	if err := db.Init(); err != nil {
		log.Fatal("数据库初始化失败:", err)
	}

	fmt.Println("开始生成测试数据...")

	// 创建测试用户
	createTestUsers()

	// 创建测试标签
	createTestTags()

	// 创建关于我页面
	createAboutPage()

	// 创建测试文章
	createTestPosts()

	fmt.Println("测试数据生成完成！")
	fmt.Println("用户: admin (密码: admin123)")
	fmt.Println("文章: 5篇测试文章")
	fmt.Println("标签: 技术、生活、思考、教程、项目")
}

// 创建测试用户
func createTestUsers() {
	// 检查是否已存在用户
	var count int64
	db.DB.Model(&db.User{}).Count(&count)
	if count > 0 {
		fmt.Println("用户已存在，跳过创建")
		return
	}

	// 创建管理员用户
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	admin := db.User{
		Username: "admin",
		Password: string(hashedPassword),
	}
	db.DB.Create(&admin)

	// 创建普通用户
	hashedPassword2, _ := bcrypt.GenerateFromPassword([]byte("user123"), bcrypt.DefaultCost)
	user := db.User{
		Username: "testuser",
		Password: string(hashedPassword2),
	}
	db.DB.Create(&user)

	fmt.Println("✅ 测试用户创建完成")
}

// 创建测试标签
func createTestTags() {
	// 检查是否已存在标签
	var count int64
	db.DB.Model(&db.Tag{}).Count(&count)
	if count > 0 {
		fmt.Println("标签已存在，跳过创建")
		return
	}

	tags := []string{"技术", "生活", "思考", "教程", "项目", "Go", "Web开发", "数据库"}
	for _, tagName := range tags {
		tag := db.Tag{Name: tagName}
		db.DB.Create(&tag)
	}

	fmt.Println("✅ 测试标签创建完成")
}

// 创建关于我页面
func createAboutPage() {
	var count int64
	db.DB.Model(&db.Page{}).Where("slug = ?", "about").Count(&count)
	if count > 0 {
		fmt.Println("关于页已存在，跳过创建")
		return
	}

	page := db.Page{
		Slug:    "about",
		Title:   "关于我",
		Summary: "AI 全栈工程师，专注技术与产品的长期主义者。",
		Content: "## 你好，我是 CommitLog\n\n- 专注 Go、前端与 AI 协同开发\n- 坚持通过文字记录成长，分享工程实践\n- 相信长期主义与复利思维\n\n### 近期关注\n1. AI 辅助研发流程的落地\n2. 优雅的技术写作与知识管理\n3. 个人品牌与产品化能力",
	}

	if err := db.DB.Create(&page).Error; err != nil {
		log.Printf("创建关于页失败: %v", err)
		return
	}

	fmt.Println("✅ 关于我页面创建完成")
}

// 创建测试文章
func createTestPosts() {
	// 检查是否已存在文章
	var count int64
	db.DB.Model(&db.Post{}).Count(&count)
	if count > 0 {
		fmt.Println("文章已存在，跳过创建")
		return
	}

	// 获取管理员用户
	var admin db.User
	db.DB.Where("username = ?", "admin").First(&admin)

	// 获取所有标签
	var allTags []db.Tag
	db.DB.Find(&allTags)

	// 创建标签映射
	tagMap := make(map[string]db.Tag)
	for _, tag := range allTags {
		tagMap[tag.Name] = tag
	}

	// 文章内容
	contents := []struct {
		title   string
		content string
		summary string
		tags    []string
	}{
		{
			title:   "使用Go语言构建高性能Web服务",
			content: "Go语言因其出色的并发性能和简洁的语法，成为构建高性能Web服务的理想选择。本文将分享如何使用Go语言构建Web服务，包括框架选择、性能优化和实际案例分析。通过合理的架构设计，我们的系统能够轻松处理数千并发请求。",
			summary: "探索如何使用Go语言构建高性能的Web服务，包括框架选择、性能优化和实际案例分析。",
			tags:    []string{"技术", "Go", "Web开发"},
		},
		{
			title:   "Markdown编辑器EasyMDE集成指南",
			content: "在Web应用中集成一个功能完善的Markdown编辑器可以大大提升用户体验。本文将详细介绍如何在Web应用中集成EasyMDE Markdown编辑器，包括基本配置、高级功能和实际应用案例。",
			summary: "详细介绍如何在Web应用中集成EasyMDE Markdown编辑器，包括基本配置和高级功能。",
			tags:    []string{"教程", "Web开发"},
		},
		{
			title:   "个人知识管理系统的设计与实现",
			content: "在信息爆炸的时代，如何有效管理个人知识成为一个重要课题。本文分享个人知识管理系统的设计理念、技术选型和实现过程，包括系统架构、功能特性和技术选型等关键要素。",
			summary: "分享个人知识管理系统的设计理念、技术选型和实现过程，帮助读者构建自己的知识管理工具。",
			tags:    []string{"思考", "项目"},
		},
		{
			title:   "SQLite数据库优化实践",
			content: "SQLite作为轻量级数据库，在很多场景下都有出色表现。本文分享SQLite数据库的优化实践经验，包括索引优化、查询优化、连接池配置和事务处理等实用技巧。",
			summary: "分享SQLite数据库的优化实践经验，包括索引优化、查询优化和连接池配置等实用技巧。",
			tags:    []string{"数据库", "技术"},
		},
		{
			title:   "现代Web开发技术栈选择思考",
			content: "在选择技术栈时，需要综合考虑项目需求、团队能力、维护成本等多个因素。本文探讨现代Web开发中技术栈选择的关键因素，分享CommitLog项目的技术选型和实际应用效果。",
			summary: "探讨现代Web开发中技术栈选择的关键因素，分享CommitLog项目的技术选型和实际应用效果。",
			tags:    []string{"技术", "Web开发", "思考"},
		},
	}

	// 创建文章
	for _, data := range contents {
		post := db.Post{
			Title:       data.title,
			Content:     data.content,
			Summary:     data.summary,
			Status:      "published",
			UserID:      admin.ID,
			ReadingTime: len(data.content) / 200,
		}
		if post.ReadingTime < 1 {
			post.ReadingTime = 1
		}

		// 创建文章
		if err := db.DB.Create(&post).Error; err != nil {
			log.Printf("创建文章失败: %v", err)
			continue
		}

		// 关联标签
		var postTags []db.Tag
		for _, tagName := range data.tags {
			if tag, ok := tagMap[tagName]; ok {
				postTags = append(postTags, tag)
			}
		}

		if len(postTags) > 0 {
			if err := db.DB.Model(&post).Association("Tags").Append(postTags); err != nil {
				log.Printf("关联标签失败: %v", err)
			}
		}
	}

	fmt.Println("✅ 测试文章创建完成")
}
