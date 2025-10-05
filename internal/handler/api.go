package handler

import (
	"github.com/commitlog/internal/service"
	"gorm.io/gorm"
)

// API bundles shared dependencies for HTTP handlers.
type API struct {
	db        *gorm.DB
	posts     *service.PostService
	tags      *service.TagService
	pages     *service.PageService
	habits    *service.HabitService
	habitLogs *service.HabitLogService
	profiles  *service.ProfileService
	analytics *service.AnalyticsService
	system    *service.SystemSettingService
	summaries service.SummaryGenerator
	uploadDir string
	uploadURL string
}

// NewAPI constructs a handler set with shared services.
func NewAPI(db *gorm.DB, uploadDir, uploadURL string) *API {
	systemService := service.NewSystemSettingService(db)
	summaryService := service.NewAISummaryService(systemService)

	return &API{
		db:        db,
		posts:     service.NewPostService(db),
		tags:      service.NewTagService(db),
		pages:     service.NewPageService(db),
		habits:    service.NewHabitService(db),
		habitLogs: service.NewHabitLogService(db),
		profiles:  service.NewProfileService(db),
		analytics: service.NewAnalyticsService(db),
		system:    systemService,
		summaries: summaryService,
		uploadDir: uploadDir,
		uploadURL: uploadURL,
	}
}

// DB exposes the underlying gorm instance for legacy paths.
func (a *API) DB() *gorm.DB {
	return a.db
}
