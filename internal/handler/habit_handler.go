package handler

import (
	"cmp"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
	"github.com/gin-gonic/gin"
	"slices"
)

const (
	defaultHabitView = "monthly"
	dateFormat       = "2006-01-02"
)

type heatmapHabit struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	TypeTag string `json:"type_tag"`
}

type heatmapDay struct {
	Date   string         `json:"date"`
	Habits []heatmapHabit `json:"habits"`
}

type heatmapRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type heatmapSummary struct {
	TotalLogs  int `json:"total_logs"`
	ActiveDays int `json:"active_days"`
	HabitCount int `json:"habit_count"`
}

type habitHeatmapPayload struct {
	Range       heatmapRange   `json:"range"`
	Days        []heatmapDay   `json:"days"`
	Habits      []heatmapHabit `json:"habits"`
	Summary     heatmapSummary `json:"summary"`
	GeneratedAt string         `json:"generated_at"`
}

type habitPayload struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	FrequencyUnit  string `json:"frequency_unit"`
	FrequencyCount int    `json:"frequency_count"`
	TypeTag        string `json:"type_tag"`
	Status         string `json:"status"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
}

// ShowHabitList 渲染习惯列表页面
func (a *API) ShowHabitList(c *gin.Context) {
	filter := service.HabitFilter{
		Status:  c.Query("status"),
		TypeTag: c.Query("type"),
		Search:  c.Query("search"),
	}

	habits, err := a.habits.List(filter)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "habit_list.html", gin.H{
			"title": "习惯管理",
			"error": "获取习惯列表失败",
		})
		return
	}

	typeTags := uniqueHabitTags(habits)
	var selectedHabitID uint
	if len(habits) > 0 {
		selectedHabitID = habits[0].ID
	}

	c.HTML(http.StatusOK, "habit_list.html", gin.H{
		"title":           "习惯管理",
		"habits":          habits,
		"filter":          filter,
		"typeTags":        typeTags,
		"selectedHabitID": selectedHabitID,
	})
}

// ListHabits 返回习惯列表 JSON
func (a *API) ListHabits(c *gin.Context) {
	filter := service.HabitFilter{
		Status:  c.Query("status"),
		TypeTag: c.Query("type_tag"),
		Search:  c.Query("search"),
	}

	habits, err := a.habits.List(filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取习惯列表失败")
		return
	}

	items := make([]gin.H, 0, len(habits))
	for _, habit := range habits {
		items = append(items, habitToPayload(habit))
	}

	respondHabitSuccess(c, http.StatusOK, gin.H{"habits": items})
}

// ShowHabitEdit 渲染创建/编辑习惯页
func (a *API) ShowHabitEdit(c *gin.Context) {
	data := gin.H{
		"title": "创建习惯",
		"habit": db.Habit{FrequencyUnit: "daily", FrequencyCount: 1, Status: "active"},
	}

	if idParam := c.Param("id"); idParam != "" {
		if id, err := strconv.ParseUint(idParam, 10, 32); err == nil {
			habit, err := a.habits.Get(uint(id))
			if err == nil {
				data["title"] = "编辑习惯"
				data["habit"] = habit
			} else if errors.Is(err, service.ErrHabitNotFound) {
				data["error"] = "习惯不存在"
			} else {
				data["error"] = "加载习惯失败"
			}
		}
	}

	c.HTML(http.StatusOK, "habit_edit.html", data)
}

// GetHabit 返回单个习惯详情
func (a *API) GetHabit(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的习惯ID")
		return
	}

	habit, err := a.habits.Get(id)
	if err != nil {
		handleHabitError(c, err)
		return
	}

	respondHabitSuccess(c, http.StatusOK, gin.H{"habit": habitToPayload(*habit)})
}

// GetHabitHeatmap 返回过去一年的习惯打卡热力图
func (a *API) GetHabitHeatmap(c *gin.Context) {
	now := time.Now().In(time.Local)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := end.AddDate(0, 0, -364)

	entries, err := a.habitLogs.HeatmapRange(start, end)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取热力图数据失败")
		return
	}

	payload := buildHabitHeatmapPayload(entries, start, end, now)
	respondHabitSuccess(c, http.StatusOK, payload)
}

func buildHabitHeatmapPayload(entries []service.HabitHeatmapEntry, start, end, generatedAt time.Time) habitHeatmapPayload {
	dayMap := make(map[string][]heatmapHabit)
	legendMap := make(map[uint]heatmapHabit)

	for _, entry := range entries {
		habit := heatmapHabit{ID: entry.HabitID, Name: entry.HabitName, TypeTag: entry.HabitType}
		key := entry.LogDate.Format(dateFormat)
		dayMap[key] = append(dayMap[key], habit)
		if _, exists := legendMap[habit.ID]; !exists {
			legendMap[habit.ID] = habit
		}
	}

	days := make([]heatmapDay, 0, len(dayMap))
	for date, habits := range dayMap {
		slices.SortFunc(habits, func(a, b heatmapHabit) int {
			return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
		days = append(days, heatmapDay{Date: date, Habits: habits})
	}

	slices.SortFunc(days, func(a, b heatmapDay) int {
		return cmp.Compare(a.Date, b.Date)
	})

	legend := make([]heatmapHabit, 0, len(legendMap))
	for _, item := range legendMap {
		legend = append(legend, item)
	}

	slices.SortFunc(legend, func(a, b heatmapHabit) int {
		if diff := cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); diff != 0 {
			return diff
		}
		return cmp.Compare(a.ID, b.ID)
	})

	payload := habitHeatmapPayload{
		Range: heatmapRange{
			Start: start.Format(dateFormat),
			End:   end.Format(dateFormat),
		},
		Days:    days,
		Habits:  legend,
		Summary: heatmapSummary{TotalLogs: len(entries), ActiveDays: len(dayMap), HabitCount: len(legend)},
	}

	if !generatedAt.IsZero() {
		payload.GeneratedAt = generatedAt.Format(time.RFC3339)
	}

	return payload
}

// CreateHabit 创建习惯
func (a *API) CreateHabit(c *gin.Context) {
	input, ok := a.parseHabitInput(c)
	if !ok {
		return
	}

	habit, err := a.habits.Create(input)
	if err != nil {
		handleHabitError(c, err)
		return
	}

	respondHabitSuccess(c, http.StatusOK, gin.H{"habit": habitToPayload(*habit)})
}

// UpdateHabit 更新习惯
func (a *API) UpdateHabit(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的习惯ID")
		return
	}

	input, ok := a.parseHabitInput(c)
	if !ok {
		return
	}

	habit, err := a.habits.Update(id, input)
	if err != nil {
		handleHabitError(c, err)
		return
	}

	respondHabitSuccess(c, http.StatusOK, gin.H{"habit": habitToPayload(*habit)})
}

// DeleteHabit 删除习惯
func (a *API) DeleteHabit(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的习惯ID")
		return
	}

	if err := a.habits.Delete(id); err != nil {
		respondError(c, http.StatusInternalServerError, "删除习惯失败")
		return
	}

	respondHabitSuccess(c, http.StatusOK, gin.H{"deleted": true})
}

// GetHabitCalendar 返回日期区间内的打卡数据和统计
func (a *API) GetHabitCalendar(c *gin.Context) {
	habitID, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的习惯ID")
		return
	}

	habit, err := a.habits.Get(habitID)
	if err != nil {
		if errors.Is(err, service.ErrHabitNotFound) {
			respondError(c, http.StatusNotFound, "习惯不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "加载习惯失败")
		return
	}

	view := c.DefaultQuery("view", defaultHabitView)
	start, end := resolveRange(c.Query("start"), view)

	logs, err := a.habitLogs.ListBetween(service.HabitLogFilter{HabitID: habit.ID, Start: start, End: end})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取打卡记录失败")
		return
	}

	stats, err := a.habitLogs.StatsBetween(service.HabitLogFilter{HabitID: habit.ID, Start: start, End: end}, *habit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "计算统计信息失败")
		return
	}

	payload := gin.H{
		"habit": habitToPayload(*habit),
		"logs":  serializeHabitLogs(logs),
		"stats": serializeHabitStats(stats),
		"range": gin.H{"start": start.Format(dateFormat), "end": end.Format(dateFormat), "view": view},
	}

	respondHabitSuccess(c, http.StatusOK, payload)
}

// QuickLogHabit 提供后台快速打卡能力
func (a *API) QuickLogHabit(c *gin.Context) {
	habitID, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的习惯ID")
		return
	}

	var payload struct {
		LogDate string `json:"log_date"` // 2006-01-02
		LogTime string `json:"log_time"` // 15:04，可选
		Note    string `json:"note"`
	}

	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		if !bindJSON(c, &payload, "请求参数不合法") {
			return
		}
	} else {
		payload.LogDate = c.PostForm("log_date")
		payload.LogTime = c.PostForm("log_time")
		payload.Note = c.PostForm("note")
	}

	if payload.LogDate == "" {
		respondError(c, http.StatusBadRequest, "请选择打卡日期")
		return
	}

	logDate, err := time.ParseInLocation(dateFormat, payload.LogDate, time.Local)
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的打卡日期")
		return
	}

	var logTimePtr *time.Time
	if payload.LogTime != "" {
		if t, err := time.ParseInLocation("15:04", payload.LogTime, time.Local); err == nil {
			combined := time.Date(logDate.Year(), logDate.Month(), logDate.Day(), t.Hour(), t.Minute(), 0, 0, logDate.Location())
			logTimePtr = &combined
		} else {
			respondError(c, http.StatusBadRequest, "无效的打卡时间")
			return
		}
	}

	logEntry, err := a.habitLogs.Upsert(service.HabitLogInput{
		HabitID: habitID,
		LogDate: logDate,
		LogTime: logTimePtr,
		Note:    payload.Note,
		Source:  "admin_manual",
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "保存打卡记录失败")
		return
	}

	respondHabitSuccess(c, http.StatusOK, gin.H{"log": serializeHabitLog(*logEntry)})
}

// DeleteHabitLog 删除单条打卡
func (a *API) DeleteHabitLog(c *gin.Context) {
	habitID, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的习惯ID")
		return
	}

	logID, err := parseUintParam(c, "logId")
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的打卡记录ID")
		return
	}

	if err := a.habitLogs.Delete(logID); err != nil {
		respondError(c, http.StatusInternalServerError, "删除打卡记录失败")
		return
	}

	respondHabitSuccess(c, http.StatusOK, gin.H{"deleted": true, "habit_id": habitID})
}

func (a *API) parseHabitInput(c *gin.Context) (service.HabitInput, bool) {
	var payload habitPayload

	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		if !bindJSON(c, &payload, "请求参数不合法") {
			return service.HabitInput{}, false
		}
	} else {
		payload.Name = c.PostForm("name")
		payload.Description = c.PostForm("description")
		payload.FrequencyUnit = c.PostForm("frequency_unit")
		payload.TypeTag = c.PostForm("type_tag")
		payload.Status = c.PostForm("status")
		payload.StartDate = c.PostForm("start_date")
		payload.EndDate = c.PostForm("end_date")

		if countStr := c.PostForm("frequency_count"); countStr != "" {
			if val, err := strconv.Atoi(countStr); err == nil {
				payload.FrequencyCount = val
			} else {
				respondError(c, http.StatusBadRequest, "目标频率应为数字")
				return service.HabitInput{}, false
			}
		}
	}

	startPtr, ok := parseOptionalDate(payload.StartDate)
	if !ok {
		respondError(c, http.StatusBadRequest, "无效的开始日期")
		return service.HabitInput{}, false
	}
	endPtr, ok := parseOptionalDate(payload.EndDate)
	if !ok {
		respondError(c, http.StatusBadRequest, "无效的结束日期")
		return service.HabitInput{}, false
	}

	input := service.HabitInput{
		Name:           payload.Name,
		Description:    payload.Description,
		FrequencyUnit:  payload.FrequencyUnit,
		FrequencyCount: payload.FrequencyCount,
		TypeTag:        payload.TypeTag,
		Status:         payload.Status,
		StartDate:      startPtr,
		EndDate:        endPtr,
	}

	if input.FrequencyCount == 0 {
		respondError(c, http.StatusBadRequest, "目标频率不能为空")
		return service.HabitInput{}, false
	}

	return input, true
}

func parseOptionalDate(value string) (*time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}

	t, err := time.ParseInLocation(dateFormat, value, time.Local)
	if err != nil {
		return nil, false
	}

	return &t, true
}

func uniqueHabitTags(habits []db.Habit) []string {
	tagMap := make(map[string]struct{})

	for _, habit := range habits {
		tag := strings.TrimSpace(habit.TypeTag)
		if tag == "" {
			continue
		}
		tagMap[tag] = struct{}{}
	}

	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	slices.Sort(tags)
	return tags
}

func habitToPayload(habit db.Habit) gin.H {
	item := gin.H{
		"id":              habit.ID,
		"name":            habit.Name,
		"description":     habit.Description,
		"frequency_unit":  habit.FrequencyUnit,
		"frequency_count": habit.FrequencyCount,
		"type_tag":        habit.TypeTag,
		"status":          habit.Status,
	}

	if habit.StartDate != nil {
		item["start_date"] = habit.StartDate.Format(dateFormat)
	}
	if habit.EndDate != nil {
		item["end_date"] = habit.EndDate.Format(dateFormat)
	}

	return item
}

func serializeHabitLogs(logs []db.HabitLog) []gin.H {
	items := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		item := gin.H{
			"id":       log.ID,
			"habit_id": log.HabitID,
			"log_date": log.LogDate.Format(dateFormat),
			"source":   log.Source,
			"note":     log.Note,
		}
		if log.LogTime != nil {
			item["log_time"] = log.LogTime.Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items
}

func serializeHabitLog(log db.HabitLog) gin.H {
	payload := gin.H{
		"id":       log.ID,
		"habit_id": log.HabitID,
		"log_date": log.LogDate.Format(dateFormat),
		"source":   log.Source,
		"note":     log.Note,
	}
	if log.LogTime != nil {
		payload["log_time"] = log.LogTime.Format(time.RFC3339)
	}
	return payload
}

func serializeHabitStats(stats *service.HabitStats) gin.H {
	return gin.H{
		"range_start":     stats.RangeStart.Format(dateFormat),
		"range_end":       stats.RangeEnd.Format(dateFormat),
		"completed_count": stats.CompletedCount,
		"target_count":    stats.TargetCount,
		"completion_rate": stats.CompletionRate,
		"current_streak":  stats.CurrentStreak,
		"longest_streak":  stats.LongestStreak,
	}
}

func respondHabitSuccess(c *gin.Context, status int, payload any) {
	c.JSON(status, payload)
}

func resolveRange(startStr, view string) (time.Time, time.Time) {
	var start time.Time
	var err error

	if startStr != "" {
		start, err = time.ParseInLocation(dateFormat, startStr, time.Local)
	}
	if err != nil || startStr == "" {
		today := time.Now()
		start = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	}

	switch strings.ToLower(view) {
	case "weekly":
		weekday := int(start.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = start.AddDate(0, 0, -weekday+1)
		end := start.AddDate(0, 0, 6)
		return start, end
	default:
		start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
		end := start.AddDate(0, 1, -1)
		return start, end
	}
}

func handleHabitError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrHabitNotFound):
		respondError(c, http.StatusNotFound, "习惯不存在")
	case errors.Is(err, service.ErrHabitInvalidFrequency):
		respondError(c, http.StatusBadRequest, "频率配置无效")
	default:
		respondError(c, http.StatusInternalServerError, "操作失败")
	}
}
