package handler

import "github.com/commitlog/internal/locale"

var fixedTitleMap = map[string]string{
	"首页":     "Home",
	"文章详情":   "Post Details",
	"标签":     "Tags",
	"关于":     "About",
	"摄影作品":   "Gallery",
	"管理员登录": "Admin Login",
	"管理面板":   "Dashboard",
	"系统设置":   "System Settings",
	"标签管理":   "Tag Management",
	"文章管理":   "Post Management",
	"创建文章":   "Create Post",
	"编辑文章":   "Edit Post",
	"关于我":   "About Me",
}

func localizeFixedTitle(language, title string) string {
	if title == "" {
		return title
	}
	normalized := locale.NormalizeLanguage(language)
	if normalized == locale.LanguageEnglish {
		if mapped, ok := fixedTitleMap[title]; ok {
			return mapped
		}
		return title
	}
	for key, value := range fixedTitleMap {
		if value == title {
			return key
		}
	}
	return title
}
