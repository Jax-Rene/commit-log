package handler

import (
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostHandler 处理文章相关的请求

// GetPosts 获取文章列表
func GetPosts(c *gin.Context) {
	var posts []db.Post
	if err := db.DB.Preload("Tags").Order("created_at desc").Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文章列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// GetPost 获取单篇文章
func GetPost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	var post db.Post
	if err := db.DB.Preload("Tags").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": post})
}

// CreatePost 创建新文章
func CreatePost(c *gin.Context) {
	// 解析请求体中的数据
	var postData struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Summary string   `json:"summary"`
		Status  string   `json:"status"`
		Tags    []db.Tag `json:"tags"`
	}

	if err := c.ShouldBindJSON(&postData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 创建新文章
	post := db.Post{
		Title:   postData.Title,
		Content: postData.Content,
		Summary: postData.Summary,
		Status:  postData.Status,
		UserID:  1, // 默认用户ID，实际应从会话中获取
	}

	// 开启事务
	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建文章
	if err := tx.Create(&post).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文章失败"})
		return
	}

	// 处理标签
	if len(postData.Tags) > 0 {
		for i := range postData.Tags {
			var tag db.Tag
			// 查找或创建标签
			result := tx.Where("name = ?", postData.Tags[i].Name).FirstOrCreate(&tag)
			if result.Error != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "处理标签失败"})
				return
			}
			postData.Tags[i] = tag
		}

		// 添加标签关联
		if err := tx.Model(&post).Association("Tags").Append(postData.Tags); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "添加标签关联失败"})
			return
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交事务失败"})
		return
	}

	// 重新加载文章及其关联的标签
	db.DB.Preload("Tags").First(&post, post.ID)

	c.JSON(http.StatusOK, gin.H{"message": "文章创建成功", "post": post})
}

// UpdatePost 更新文章
func UpdatePost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	// 先获取现有文章
	var existingPost db.Post
	if err := db.DB.First(&existingPost, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	// 解析请求体中的更新数据
	var updateData struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Summary string   `json:"summary"`
		Status  string   `json:"status"`
		Tags    []db.Tag `json:"tags"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新文章字段
	existingPost.Title = updateData.Title
	existingPost.Content = updateData.Content
	existingPost.Summary = updateData.Summary
	existingPost.Status = updateData.Status

	// 开启事务处理标签和文章更新
	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 更新文章
	if err := tx.Save(&existingPost).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新文章失败"})
		return
	}

	// 处理标签
	if len(updateData.Tags) > 0 {
		// 清除现有标签关联
		if err := tx.Model(&existingPost).Association("Tags").Clear(); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新标签失败"})
			return
		}

		// 处理新标签
		for i := range updateData.Tags {
			var tag db.Tag
			// 查找或创建标签
			result := tx.Where("name = ?", updateData.Tags[i].Name).FirstOrCreate(&tag)
			if result.Error != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "处理标签失败"})
				return
			}
			updateData.Tags[i] = tag
		}

		// 添加新标签关联
		if err := tx.Model(&existingPost).Association("Tags").Append(updateData.Tags); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "添加标签关联失败"})
			return
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交事务失败"})
		return
	}

	// 重新加载文章及其关联的标签
	db.DB.Preload("Tags").First(&existingPost, id)

	c.JSON(http.StatusOK, gin.H{"message": "文章更新成功", "post": existingPost})
}

// DeletePost 删除文章
func DeletePost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	if err := db.DB.Delete(&db.Post{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文章失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文章删除成功"})
}

// ShowPostList 渲染文章管理列表页面
func ShowPostList(c *gin.Context) {
	// 获取查询参数
	page := 1
	perPage := 10
	search := c.Query("search")
	status := c.Query("status")
	tags := c.QueryArray("tags")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	// 打印接收到的参数用于调试
	log.Printf("[DEBUG] ShowPostList - 接收到的参数:")
	log.Printf("  - page: %d", page)
	log.Printf("  - search: %q", search)
	log.Printf("  - status: %q", status)
	log.Printf("  - tags: %v", tags)
	log.Printf("  - start_date: %q", startDate)
	log.Printf("  - end_date: %q", endDate)

	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
		log.Printf("  - 解析后的page: %d", page)
	}

	// 开启GORM调试模式
	db.DB = db.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Info)})

	// 基础查询
	log.Printf("[DEBUG] 开始构建查询...")
	baseQuery := db.DB.Model(&db.Post{}).Preload("Tags").Preload("User")

	// 构建筛选查询（用于获取总数和统计数据）
	filterQuery := baseQuery.Session(&gorm.Session{})

	// 应用筛选条件到基础查询
	applyFilters := func(query *gorm.DB) *gorm.DB {
		// 搜索条件
		if search != "" {
			searchQuery := "%" + search + "%"
			query = query.Where("posts.title LIKE ? OR posts.content LIKE ? OR posts.summary LIKE ?", searchQuery, searchQuery, searchQuery)
			log.Printf("[DEBUG] 应用搜索条件: %q", search)
		}

		// 状态筛选
		if status != "" {
			query = query.Where("posts.status = ?", status)
			log.Printf("[DEBUG] 应用状态筛选: %q", status)
		}

		// 标签筛选
		if len(tags) > 0 {
			log.Printf("[DEBUG] 应用标签筛选: %v", tags)
			query = query.Joins("JOIN post_tags ON posts.id = post_tags.post_id").
				Joins("JOIN tags ON tags.id = post_tags.tag_id").
				Where("posts.id IN (?)", db.DB.Model(&db.Post{}).
					Select("posts.id").
					Joins("JOIN post_tags ON posts.id = post_tags.post_id").
					Joins("JOIN tags ON tags.id = post_tags.tag_id").
					Where("tags.name IN ?", tags))
		}

		// 时间范围筛选
		if startDate != "" {
			if start, err := time.Parse("2006-01-02", startDate); err == nil {
				query = query.Where("posts.created_at >= ?", start)
				log.Printf("[DEBUG] 应用开始日期: %v", start)
			} else {
				log.Printf("[ERROR] 解析开始日期失败: %v", err)
			}
		}
		if endDate != "" {
			if end, err := time.Parse("2006-01-02", endDate); err == nil {
				// 将结束时间设为当天的23:59:59
				end = end.Add(24*time.Hour - time.Second)
				query = query.Where("posts.created_at <= ?", end)
				log.Printf("[DEBUG] 应用结束日期: %v", end)
			} else {
				log.Printf("[ERROR] 解析结束日期失败: %v", err)
			}
		}
		return query
	}

	// 应用筛选到基础查询
	filterQuery = applyFilters(filterQuery)

	// 获取统计数据
	var total, publishedCount, draftCount int64
	log.Printf("[DEBUG] 开始计算统计数据...")

	// 总记录数
	filterQuery.Count(&total)
	log.Printf("[DEBUG] 总记录数: %d", total)

	// 已发布文章数量 - 应用相同的筛选条件
	publishedQuery := baseQuery.Session(&gorm.Session{})
	publishedQuery = applyFilters(publishedQuery)
	publishedQuery.Where("status = ?", "published").Count(&publishedCount)
	log.Printf("[DEBUG] 已发布文章数(筛选条件下): %d", publishedCount)

	// 草稿文章数量 - 应用相同的筛选条件
	draftQuery := baseQuery.Session(&gorm.Session{})
	draftQuery = applyFilters(draftQuery)
	draftQuery.Where("status = ?", "draft").Count(&draftCount)
	log.Printf("[DEBUG] 草稿文章数(筛选条件下): %d", draftCount)

	// 计算分页信息
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages < 1 {
		totalPages = 1
	}
	log.Printf("[DEBUG] 总页数: %d (每页%d条)", totalPages, perPage)

	// 计算分页
	offset := (page - 1) * perPage
	log.Printf("[DEBUG] 分页参数: offset=%d, limit=%d, page=%d", offset, perPage, page)

	// 获取文章列表（应用相同的筛选条件）
	var posts []db.Post
	log.Printf("[DEBUG] 开始执行查询...")
	listQuery := applyFilters(baseQuery)
	err := listQuery.Order("posts.created_at desc").Limit(perPage).Offset(offset).Find(&posts).Error

	if err != nil {
		log.Printf("[ERROR] 查询文章失败: %v", err)
		c.HTML(http.StatusInternalServerError, "post_list.html", gin.H{
			"title": "文章管理",
			"error": "获取文章列表失败",
		})
		return
	}

	log.Printf("[DEBUG] 查询完成，返回 %d 篇文章", len(posts))

	// 获取所有标签用于筛选
	var allTags []db.Tag
	db.DB.Find(&allTags)
	log.Printf("[DEBUG] 获取标签列表: %d 个标签", len(allTags))

	// 生成分页数字范围
	var pages []int
	for i := 1; i <= totalPages; i++ {
		pages = append(pages, i)
	}

	// 构建查询参数字符串（使用URL编码确保特殊字符正确处理）
	queryParams := ""
	if search != "" {
		queryParams += "&search=" + url.QueryEscape(search)
	}
	if status != "" {
		queryParams += "&status=" + url.QueryEscape(status)
	}
	if startDate != "" {
		queryParams += "&start_date=" + url.QueryEscape(startDate)
	}
	if endDate != "" {
		queryParams += "&end_date=" + url.QueryEscape(endDate)
	}
	for _, tag := range tags {
		queryParams += "&tags=" + url.QueryEscape(tag)
	}

	log.Printf("[DEBUG] 准备渲染模板 - 参数: page=%d, total=%d, totalPages=%d, published=%d, draft=%d",
		page, total, totalPages, publishedCount, draftCount)

	c.HTML(http.StatusOK, "post_list.html", gin.H{
		"title":          "文章管理",
		"posts":          posts,
		"allTags":        allTags,
		"search":         search,
		"status":         status,
		"tags":           tags,
		"startDate":      startDate,
		"endDate":        endDate,
		"page":           page,
		"perPage":        perPage,
		"total":          total,
		"totalPages":     totalPages,
		"publishedCount": publishedCount,
		"draftCount":     draftCount,
		"pages":          pages,
		"queryParams":    queryParams,
	})
}

// ShowPostEdit 渲染文章编辑页面
func ShowPostEdit(c *gin.Context) {
	id := c.Param("id")

	data := gin.H{
		"title": "编辑文章",
	}

	if id != "" {
		// 编辑现有文章
		var post db.Post
		if err := db.DB.Preload("Tags").First(&post, id).Error; err == nil {
			data["post"] = post
		}
	} else {
		// 创建新文章
		data["title"] = "创建文章"
	}

	c.HTML(http.StatusOK, "post_edit.html", data)
}
