package db

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// markdownEmphasisReplacer 用于去除 Markdown 斜体和粗体标记。
var markdownEmphasisReplacer = strings.NewReplacer("**", "", "*", "")

// Post 定义了文章模型
type Post struct {
	gorm.Model
	Content            string
	Summary            string
	Status             string `gorm:"default:draft"` // draft, published
	Language           string `gorm:"size:8;index;default:zh"`
	ReadingTime        int
	CoverURL           string
	CoverWidth         int
	CoverHeight        int
	UserID             uint
	User               User
	Tags               []Tag `gorm:"many2many:post_tags;"`
	PublishedAt        time.Time
	TranslationGroupID uint `gorm:"index;default:0"`
	// PublicationCount 记录文章发布次数，用于版本号展示
	PublicationCount int
	// LatestPublicationID 指向最近一次发布的快照
	LatestPublicationID *uint
	// Title 是根据 Content 动态推导出的字段，不在数据库中存储
	Title string `gorm:"-"`
}

// PostPublication 存储文章发布时的快照数据
type PostPublication struct {
	gorm.Model
	PostID      uint
	Post        Post
	Content     string
	Summary     string
	ReadingTime int
	CoverURL    string
	CoverWidth  int
	CoverHeight int
	UserID      uint
	User        User
	PublishedAt time.Time
	Version     int
	Tags        []Tag `gorm:"many2many:post_publication_tags;"`
	// Title 与 Post.Title 一样由 Content 动态生成
	Title string `gorm:"-"`
}

// PopulateDerivedFields 根据内容动态生成标题等衍生信息。
func (p *Post) PopulateDerivedFields() {
	p.Title = DeriveTitleFromContent(p.Content)
}

// AfterFind 在查询后填充衍生字段。
func (p *Post) AfterFind(tx *gorm.DB) error {
	p.PopulateDerivedFields()
	return nil
}

// PopulateDerivedFields 根据内容动态生成标题等衍生信息。
func (pp *PostPublication) PopulateDerivedFields() {
	pp.Title = DeriveTitleFromContent(pp.Content)
}

// AfterFind 在查询后填充衍生字段。
func (pp *PostPublication) AfterFind(tx *gorm.DB) error {
	pp.PopulateDerivedFields()
	return nil
}

// DeriveTitleFromContent 读取内容首行并去除 Markdown 语法以生成标题。
func DeriveTitleFromContent(content string) string {
	if content == "" {
		return ""
	}

	firstLine := content
	if idx := strings.IndexRune(content, '\n'); idx >= 0 {
		firstLine = content[:idx]
	}

	trimmed := strings.TrimSpace(firstLine)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "#") {
		trimmed = strings.TrimLeft(trimmed, "#")
		trimmed = strings.TrimSpace(trimmed)
		trimmed = strings.TrimRight(trimmed, "#")
		trimmed = strings.TrimSpace(trimmed)
	}

	trimmed = stripMarkdownEmphasis(trimmed)
	trimmed = strings.TrimSpace(trimmed)

	return trimmed
}

// stripMarkdownEmphasis 去除 Markdown 斜体和粗体标记。
func stripMarkdownEmphasis(text string) string {
	return markdownEmphasisReplacer.Replace(text)
}
