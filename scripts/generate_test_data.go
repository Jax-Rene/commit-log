package main

import (
	"fmt"
	"log"
	"time"

	"github.com/commitlog/internal/config"
	"github.com/commitlog/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// 测试数据生成器
func main() {
	// 初始化数据库
	cfg := config.Load()
	if err := db.Init(cfg.DatabasePath); err != nil {
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
	// 清理旧文章及关联
	db.DB.Exec("DELETE FROM post_publication_tags")
	db.DB.Exec("DELETE FROM post_publications")
	db.DB.Exec("DELETE FROM post_tags")
	db.DB.Exec("DELETE FROM posts")

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
		title       string
		content     string
		summary     string
		tags        []string
		coverURL    string
		coverWidth  int
		coverHeight int
	}{
		{
			title:       "使用Go语言构建高性能Web服务",
			content:     "Go语言因其出色的并发性能和简洁的语法，成为构建高性能Web服务的理想选择。本文将分享如何使用Go语言构建Web服务，包括框架选择、性能优化和实际案例分析。通过合理的架构设计，我们的系统能够轻松处理数千并发请求。",
			summary:     "探索如何使用Go语言构建高性能的Web服务，包括框架选择、性能优化和实际案例分析。",
			tags:        []string{"技术", "Go", "Web开发"},
			coverURL:    "https://images.unsplash.com/photo-1523475472560-d2df97ec485c?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1050,
		},
		{
			title:       "Markdown编辑器EasyMDE集成指南",
			content:     "在Web应用中集成一个功能完善的Markdown编辑器可以大大提升用户体验。本文将详细介绍如何在Web应用中集成EasyMDE Markdown编辑器，包括基本配置、高级功能和实际应用案例。",
			summary:     "详细介绍如何在Web应用中集成EasyMDE Markdown编辑器，包括基本配置和高级功能。",
			tags:        []string{"教程", "Web开发"},
			coverURL:    "https://images.unsplash.com/photo-1517430816045-df4b7de11d1d?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "个人知识管理系统的设计与实现",
			content:     "在信息爆炸的时代，如何有效管理个人知识成为一个重要课题。本文分享个人知识管理系统的设计理念、技术选型和实现过程，包括系统架构、功能特性和技术选型等关键要素。",
			summary:     "分享个人知识管理系统的设计理念、技术选型和实现过程，帮助读者构建自己的知识管理工具。",
			tags:        []string{"思考", "项目"},
			coverURL:    "https://images.unsplash.com/photo-1523473827534-86c23bcb06b1?auto=format&fit=crop&w=1350&q=80",
			coverWidth:  1100,
			coverHeight: 1700,
		},
		{
			title:       "SQLite数据库优化实践",
			content:     "SQLite作为轻量级数据库，在很多场景下都有出色表现。本文分享SQLite数据库的优化实践经验，包括索引优化、查询优化、连接池配置和事务处理等实用技巧。",
			summary:     "分享SQLite数据库的优化实践经验，包括索引优化、查询优化和连接池配置等实用技巧。",
			tags:        []string{"数据库", "技术"},
			coverURL:    "https://images.unsplash.com/photo-1518770660439-4636190af475?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "现代Web开发技术栈选择思考",
			content:     "在选择技术栈时，需要综合考虑项目需求、团队能力、维护成本等多个因素。本文探讨现代Web开发中技术栈选择的关键因素，分享CommitLog项目的技术选型和实际应用效果。",
			summary:     "探讨现代Web开发中技术栈选择的关键因素，分享CommitLog项目的技术选型和实际应用效果。",
			tags:        []string{"技术", "Web开发", "思考"},
			coverURL:    "https://images.unsplash.com/photo-1487058792275-0ad4aaf24ca7?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "GORM使用技巧与最佳实践",
			content:     "GORM是Go语言中最流行的ORM库之一。本文总结了GORM的常用用法、性能优化建议以及在实际项目中的最佳实践，帮助开发者更高效地进行数据库操作。",
			summary:     "总结GORM的常用用法和性能优化建议，助力高效数据库开发。",
			tags:        []string{"Go", "数据库", "技术"},
			coverURL:    "https://images.unsplash.com/photo-1461749280684-dccba630e2f6?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Gin框架中间件开发实战",
			content:     "Gin作为高性能Web框架，支持灵活的中间件机制。本文介绍如何开发自定义中间件，实现日志、鉴权、限流等功能，并分享常见的中间件设计模式。",
			summary:     "介绍Gin中间件开发方法，涵盖日志、鉴权、限流等场景。",
			tags:        []string{"Go", "Web开发", "技术"},
			coverURL:    "https://images.unsplash.com/photo-1506744038136-46273834b3fb?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Tailwind CSS实用技巧",
			content:     "Tailwind CSS以其实用优先的设计理念受到前端开发者青睐。本文分享Tailwind CSS的常用技巧、组件复用方法以及在大型项目中的组织方式。",
			summary:     "分享Tailwind CSS的实用技巧和组件复用经验。",
			tags:        []string{"前端", "技术", "教程"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Alpine.js轻量级交互开发",
			content:     "Alpine.js为前端开发带来了极简的交互体验。本文介绍Alpine.js的核心用法、常见模式和与HTMX的协同实践。",
			summary:     "介绍Alpine.js的核心用法和与HTMX的协同开发经验。",
			tags:        []string{"前端", "技术", "教程"},
			coverURL:    "https://miro.medium.com/v2/resize:fit:1400/format:webp/1*nKKsacNC10lA-fB5FE7FrA.png",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "HTMX实现无刷新交互体验",
			content:     "HTMX让Web开发者能够轻松实现无刷新页面交互。本文介绍HTMX的基本原理、常用API和实际应用案例。",
			summary:     "介绍HTMX的基本原理和无刷新交互实现方法。",
			tags:        []string{"前端", "Web开发", "技术"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Go项目目录结构最佳实践",
			content:     "合理的项目结构有助于团队协作和代码维护。本文总结Go项目的常见目录结构方案，并结合实际项目给出推荐实践。",
			summary:     "总结Go项目的目录结构方案，提升团队协作效率。",
			tags:        []string{"Go", "项目", "思考"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "高效的代码审查流程设计",
			content:     "代码审查是保障代码质量的重要环节。本文探讨如何设计高效的代码审查流程，包括工具选择、评审规范和团队协作建议。",
			summary:     "探讨高效代码审查流程的设计与实践。",
			tags:        []string{"项目", "思考", "技术"},
			coverURL:    "https://miro.medium.com/v2/resize:fit:1400/format:webp/1*nKKsacNC10lA-fB5FE7FrA.png",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Go单元测试与Mock实践",
			content:     "单元测试是保障代码可靠性的基石。本文介绍Go语言的单元测试方法、Mock实现方式以及表驱动测试的优势。",
			summary:     "介绍Go单元测试和Mock的实践经验。",
			tags:        []string{"Go", "测试", "技术"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "如何写出可维护的前端代码",
			content:     "可维护性是前端开发的重要目标。本文分享前端代码可维护性的设计原则、常见反模式和优化建议。",
			summary:     "分享前端代码可维护性的设计原则和优化建议。",
			tags:        []string{"前端", "思考", "技术"},
			coverURL:    "https://miro.medium.com/v2/resize:fit:1400/format:webp/1*nKKsacNC10lA-fB5FE7FrA.png",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Go并发模式与应用场景",
			content:     "Go语言以其强大的并发能力著称。本文介绍Go常见的并发模式及其在实际项目中的应用场景。",
			summary:     "介绍Go并发模式及实际应用案例。",
			tags:        []string{"Go", "技术", "教程"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Web安全基础与防护实践",
			content:     "Web安全是开发者必须关注的话题。本文介绍常见的Web安全威胁及防护措施，包括XSS、CSRF、SQL注入等。",
			summary:     "介绍Web安全基础知识和防护实践。",
			tags:        []string{"Web开发", "技术", "教程"},
			coverURL:    "https://miro.medium.com/v2/resize:fit:1400/format:webp/1*nKKsacNC10lA-fB5FE7FrA.png",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "RESTful API设计规范",
			content:     "RESTful API设计有助于提升系统的可扩展性和可维护性。本文总结RESTful API的设计规范和常见实践。",
			summary:     "总结RESTful API的设计规范和实践经验。",
			tags:        []string{"Web开发", "技术", "教程"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "持续集成与自动化部署实践",
			content:     "持续集成和自动化部署是现代开发流程的核心。本文介绍CI/CD的基本原理、工具选择和落地实践。",
			summary:     "介绍持续集成与自动化部署的原理和实践。",
			tags:        []string{"项目", "技术", "教程"},
			coverURL:    "https://miro.medium.com/v2/resize:fit:1400/format:webp/1*nKKsacNC10lA-fB5FE7FrA.png",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Go语言中的错误处理模式",
			content:     "错误处理是Go开发中的重要话题。本文介绍Go语言的错误处理模式、常见陷阱和最佳实践。",
			summary:     "介绍Go错误处理的模式和最佳实践。",
			tags:        []string{"Go", "技术", "教程"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "高效的团队协作工具推荐",
			content:     "选择合适的协作工具能极大提升团队效率。本文推荐几款高效的团队协作工具，并分享实际使用体验。",
			summary:     "推荐高效团队协作工具及使用经验。",
			tags:        []string{"项目", "思考", "工具"},
			coverURL:    "https://miro.medium.com/v2/resize:fit:1400/format:webp/1*nKKsacNC10lA-fB5FE7FrA.png",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "数据库迁移与版本管理实践",
			content:     "数据库迁移和版本管理是保障数据一致性的关键。本文介绍数据库迁移工具的选择和实际操作流程。",
			summary:     "介绍数据库迁移与版本管理的实践经验。",
			tags:        []string{"数据库", "技术", "项目"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "前端组件化开发模式",
			content:     "组件化是现代前端开发的主流模式。本文介绍组件化开发的优势、常见实现方式和实际案例。",
			summary:     "介绍前端组件化开发的优势和实现方式。",
			tags:        []string{"前端", "技术", "教程"},
			coverURL:    "https://miro.medium.com/v2/resize:fit:1400/format:webp/1*nKKsacNC10lA-fB5FE7FrA.png",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Go泛型初体验",
			content:     "Go 1.18引入了泛型特性。本文介绍Go泛型的基本语法、使用场景和注意事项。",
			summary:     "介绍Go泛型的语法和使用场景。",
			tags:        []string{"Go", "技术", "教程"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "技术写作的结构化方法",
			content:     "结构化写作有助于提升技术文档的可读性。本文分享技术写作的结构化方法和常见模板。",
			summary:     "分享技术写作的结构化方法和模板。",
			tags:        []string{"思考", "教程", "技术"},
			coverURL:    "https://miro.medium.com/v2/resize:fit:1400/format:webp/1*nKKsacNC10lA-fB5FE7FrA.png",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "Go与前端协同开发实践",
			content:     "前后端协同开发是提升开发效率的关键。本文介绍Go与前端协同开发的常见模式和实践经验。",
			summary:     "介绍Go与前端协同开发的模式和经验。",
			tags:        []string{"Go", "前端", "项目"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "个人品牌建设的思考与实践",
			content:     "个人品牌建设有助于职业发展。本文分享个人品牌建设的思考、方法和实践案例。",
			summary:     "分享个人品牌建设的思考和实践经验。",
			tags:        []string{"思考", "项目", "教程"},
			coverURL:    "https://miro.medium.com/v2/resize:fit:1400/format:webp/1*nKKsacNC10lA-fB5FE7FrA.png",
			coverWidth:  1600,
			coverHeight: 1067,
		},
		{
			title:       "AI辅助研发流程探索",
			content:     "AI正在深刻改变研发流程。本文探讨AI在研发流程中的应用场景和落地实践。",
			summary:     "探讨AI在研发流程中的应用和实践。",
			tags:        []string{"AI", "技术", "思考"},
			coverURL:    "https://images.unsplash.com/photo-1465101046530-73398c7f28ca?auto=format&fit=crop&w=1600&q=80",
			coverWidth:  1600,
			coverHeight: 1067,
		},
	}

	// 创建文章
	for idx, data := range contents {
		post := db.Post{
			Title:       data.title,
			Content:     data.content,
			Summary:     data.summary,
			Status:      "draft",
			UserID:      admin.ID,
			ReadingTime: len(data.content) / 200,
			CoverURL:    data.coverURL,
			CoverWidth:  data.coverWidth,
			CoverHeight: data.coverHeight,
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

		publishedAt := time.Now().Add(-time.Duration(idx) * 12 * time.Hour)
		publication := db.PostPublication{
			PostID:      post.ID,
			Title:       post.Title,
			Content:     post.Content,
			Summary:     post.Summary,
			ReadingTime: post.ReadingTime,
			CoverURL:    post.CoverURL,
			CoverWidth:  post.CoverWidth,
			CoverHeight: post.CoverHeight,
			UserID:      post.UserID,
			PublishedAt: publishedAt,
			Version:     1,
		}

		if err := db.DB.Create(&publication).Error; err != nil {
			log.Printf("创建发布快照失败: %v", err)
			continue
		}

		if len(postTags) > 0 {
			if err := db.DB.Model(&publication).Association("Tags").Append(postTags); err != nil {
				log.Printf("关联发布标签失败: %v", err)
			}
		}

		if err := db.DB.Model(&post).Updates(map[string]interface{}{
			"status":                "published",
			"published_at":          publishedAt,
			"publication_count":     1,
			"latest_publication_id": publication.ID,
			"reading_time":          post.ReadingTime,
		}).Error; err != nil {
			log.Printf("更新文章发布信息失败: %v", err)
		}
	}

	fmt.Println("✅ 测试文章创建完成")
}
