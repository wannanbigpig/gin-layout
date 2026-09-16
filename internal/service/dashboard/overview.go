package dashboard

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service"
)

const notificationWSRequestPath = "/admin/v1/common/ws"

type Metric struct {
	Key     string  `json:"key"`
	Title   string  `json:"title"`
	Value   float64 `json:"value"`
	Suffix  string  `json:"suffix"`
	Compare string  `json:"compare"`
	Change  string  `json:"change"`
	Type    string  `json:"type"`
}

type ActivityItem struct {
	Key             string    `json:"key"`
	Title           string    `json:"title"`
	Desc            string    `json:"desc"`
	Time            string    `json:"time"`
	Type            string    `json:"type"`
	RequestLogID    uint      `json:"request_log_id"`
	RequestID       string    `json:"request_id"`
	OperationName   string    `json:"operation_name"`
	Method          string    `json:"method"`
	BaseURL         string    `json:"base_url"`
	OperatorID      uint      `json:"operator_id"`
	OperatorAccount string    `json:"operator_account"`
	OperatorName    string    `json:"operator_name"`
	OperationStatus int       `json:"operation_status"`
	IsHighRisk      uint8     `json:"is_high_risk"`
	HasChangeDiff   bool      `json:"has_change_diff"`
	CreatedAt       time.Time `json:"created_at"`
}

type UserLogin struct {
	LastLogin string `json:"last_login"`
	LastIP    string `json:"last_ip"`
}

type TrendData struct {
	Dates    []string `json:"dates"`
	Requests []int64  `json:"requests"`
	Errors   []int64  `json:"errors"`
	Logins   []int64  `json:"logins"`
}

// ResponseTimeBucket 表示请求耗时分桶统计中的单个区间。
type ResponseTimeBucket struct {
	Label   string `json:"label"`
	LowerMs int    `json:"lower_ms"`
	UpperMs int    `json:"upper_ms,omitempty"`
	Count   int64  `json:"count"`
}

// ResponseTimeStats 汇总请求耗时分布与平均耗时。
type ResponseTimeStats struct {
	Buckets []ResponseTimeBucket `json:"buckets"`
	AvgMs   float64              `json:"avg_ms"`
}

// ErrorCodeBucket 表示错误状态码聚合中的单条数据。
type ErrorCodeBucket struct {
	StatusCode int   `json:"status_code"`
	Count      int64 `json:"count"`
}

// StorageTypeBucket 表示文件存储按 file_type 聚合的单条数据。
type StorageTypeBucket struct {
	FileType  string `json:"file_type"`
	Count     int64  `json:"count"`
	SizeBytes int64  `json:"size_bytes"`
}

// StorageStats 汇总文件存储占用情况。
type StorageStats struct {
	TotalCount     int64               `json:"total_count"`
	TotalSizeBytes int64               `json:"total_size_bytes"`
	ByType         []StorageTypeBucket `json:"by_type"`
}

type Overview struct {
	Metrics      []Metric           `json:"metrics"`
	Activities   []ActivityItem     `json:"activities"`
	UserLogin    UserLogin          `json:"user_login"`
	Trend        *TrendData         `json:"trend"`
	ResponseTime *ResponseTimeStats `json:"response_time,omitempty"`
	ErrorCodes   []ErrorCodeBucket  `json:"error_codes,omitempty"`
	Storage      *StorageStats      `json:"storage,omitempty"`
}

type OverviewService struct {
	service.Base
}

func NewOverviewService() *OverviewService {
	return &OverviewService{}
}

func (s *OverviewService) Overview() (*Overview, error) {
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	activeUsers, err := countDistinct(db.Table("admin_login_logs").Where("deleted_at = 0 AND login_status = ? AND uid > 0 AND created_at >= ? AND created_at < ?", model.LoginStatusSuccess, todayStart, tomorrowStart), "uid")
	if err != nil {
		return nil, err
	}
	activeUsersYesterday, err := countDistinct(db.Table("admin_login_logs").Where("deleted_at = 0 AND login_status = ? AND uid > 0 AND created_at >= ? AND created_at < ?", model.LoginStatusSuccess, yesterdayStart, todayStart), "uid")
	if err != nil {
		return nil, err
	}

	requestsToday, errorsToday, err := requestStatsForRange(db, todayStart, tomorrowStart)
	if err != nil {
		return nil, err
	}
	requestsYesterday, errorsYesterday, err := requestStatsForRange(db, yesterdayStart, todayStart)
	if err != nil {
		return nil, err
	}

	taskCompletion, err := taskCompletionRate(db, todayStart, tomorrowStart)
	if err != nil {
		return nil, err
	}

	userLogin := UserLogin{}
	if s.GetAdminUserId() > 0 {
		var user model.AdminUser
		if err := db.Table("admin_user").Select("last_login,last_ip").Where("id = ? AND deleted_at = 0", s.GetAdminUserId()).First(&user).Error; err == nil {
			userLogin.LastLogin = user.LastLogin.String()
			userLogin.LastIP = user.LastIp
		}
	}

	trend, err := buildTrendData(db, todayStart)
	if err != nil {
		return nil, err
	}

	// 以下三段统计为增量扩展，失败不阻断主流程。
	responseTime := buildResponseTimeStats(db, todayStart, tomorrowStart)
	errorCodes := buildErrorCodeStats(db, todayStart, tomorrowStart)
	storage := buildStorageStats(db)
	errorChangeType := "danger"
	if errorsToday <= errorsYesterday {
		errorChangeType = "success"
	}

	return &Overview{
		Metrics: []Metric{
			{Key: "users", Title: "活跃用户", Value: float64(activeUsers), Compare: "较昨日", Change: formatChange(activeUsers, activeUsersYesterday), Type: changeType(activeUsers, activeUsersYesterday)},
			{Key: "requests", Title: "请求总量", Value: float64(requestsToday), Compare: "较昨日", Change: formatChange(requestsToday, requestsYesterday), Type: changeType(requestsToday, requestsYesterday)},
			{Key: "errors", Title: "异常告警", Value: float64(errorsToday), Compare: "较昨日", Change: formatChange(errorsToday, errorsYesterday), Type: errorChangeType},
			{Key: "tasks", Title: "任务完成率", Value: taskCompletion, Suffix: "%", Compare: "计划完成", Change: "+0.0%", Type: "primary"},
		},
		Activities:   buildActivities(db, s.GetAdminUserId()),
		UserLogin:    userLogin,
		Trend:        trend,
		ResponseTime: responseTime,
		ErrorCodes:   errorCodes,
		Storage:      storage,
	}, nil
}

func requestStatsForRange(db *gorm.DB, start time.Time, end time.Time) (int64, int64, error) {
	if db == nil {
		return 0, 0, nil
	}

	type statSum struct {
		TotalRequests int64
		ErrorRequests int64
	}
	var stat statSum
	if err := db.Table("daily_request_stats").
		Select("COALESCE(SUM(total_requests), 0) AS total_requests, COALESCE(SUM(error_requests), 0) AS error_requests").
		Where("date >= ? AND date < ?", statsDateKey(start), statsDateKey(end)).
		Scan(&stat).Error; err != nil {
		return 0, 0, err
	}
	return stat.TotalRequests, stat.ErrorRequests, nil
}

func countDistinct(db *gorm.DB, field string) (int64, error) {
	var count int64
	if err := db.Select("COUNT(DISTINCT " + field + ")").Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func taskCompletionRate(db *gorm.DB, start time.Time, end time.Time) (float64, error) {
	type taskStat struct {
		TotalCount   uint64
		SuccessCount uint64
	}
	var stat taskStat
	if err := db.Table("task_run_stats").
		Select("COALESCE(SUM(total_count), 0) AS total_count, COALESCE(SUM(success_count), 0) AS success_count").
		Where("window_start >= ? AND window_start < ?", start, end).
		Scan(&stat).Error; err != nil {
		return 0, err
	}
	if stat.TotalCount == 0 {
		return 0, nil
	}
	return math.Round(float64(stat.SuccessCount)*1000/float64(stat.TotalCount)) / 10, nil
}

func buildActivities(db *gorm.DB, operatorID uint) []ActivityItem {
	type row struct {
		ID              uint
		RequestID       string
		OperatorID      uint
		OperatorAccount string
		OperatorName    string
		OperationName   string
		Method          string
		BaseURL         string
		CreatedAt       time.Time
		OperationStatus int
		IsHighRisk      uint8
		HasChangeDiff   bool
	}
	var rows []row
	query := db.Table("request_logs").
		Select("id, request_id, operator_id, operator_account, operator_name, operation_name, method, base_url, created_at, operation_status, is_high_risk, (change_diff IS NOT NULL AND change_diff <> '' AND change_diff <> '{}') AS has_change_diff").
		Where("operation_status <> 0 OR is_high_risk = 1 OR (change_diff IS NOT NULL AND change_diff <> '' AND change_diff <> '{}') OR method IN ?", []string{"POST", "PUT", "PATCH", "DELETE"})
	if operatorID > 0 {
		// 首页"最近动态"只展示当前登录用户自己的操作记录。
		query = query.Where("operator_id = ?", operatorID)
	}
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(6).
		Scan(&rows).Error; err != nil {
		log.Warn("查询最近动态失败", zap.Error(err))
	}

	activities := make([]ActivityItem, 0, len(rows))
	for _, item := range rows {
		title := item.OperationName
		if title == "" {
			title = item.Method + " " + item.BaseURL
		}
		activityStatus := "warning"
		if item.OperationStatus == 0 {
			activityStatus = "success"
		}
		activities = append(activities, ActivityItem{
			Key:             strconv.FormatUint(uint64(item.ID), 10),
			Title:           title,
			Desc:            item.OperatorAccount,
			Time:            item.CreatedAt.Format("15:04"),
			Type:            activityStatus,
			RequestLogID:    item.ID,
			RequestID:       item.RequestID,
			OperationName:   item.OperationName,
			Method:          item.Method,
			BaseURL:         item.BaseURL,
			OperatorID:      item.OperatorID,
			OperatorAccount: item.OperatorAccount,
			OperatorName:    item.OperatorName,
			OperationStatus: item.OperationStatus,
			IsHighRisk:      item.IsHighRisk,
			HasChangeDiff:   item.HasChangeDiff,
			CreatedAt:       item.CreatedAt,
		})
	}
	return activities
}

func formatChange(current int64, previous int64) string {
	if previous == 0 {
		if current == 0 {
			return "+0.0%"
		}
		return "+100.0%"
	}
	change := (float64(current) - float64(previous)) * 100 / float64(previous)
	prefix := "+"
	if change < 0 {
		prefix = ""
	}
	return prefix + strconv.FormatFloat(math.Round(change*10)/10, 'f', 1, 64) + "%"
}

func changeType(current int64, previous int64) string {
	if current >= previous {
		return "success"
	}
	return "warning"
}

func buildTrendData(db *gorm.DB, todayStart time.Time) (*TrendData, error) {
	sevenDaysAgo := todayStart.AddDate(0, 0, -6)

	// 构建日期列表
	dates := make([]string, 7)
	for i := 0; i < 7; i++ {
		dates[i] = sevenDaysAgo.AddDate(0, 0, i).Format("01-02")
	}

	type dayCount struct {
		Date  string
		Count int64
	}

	requestMap, errorMap, err := requestTrendFromStats(db, sevenDaysAgo, todayStart.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}

	dateExpr := requestLogDateExpr(db)

	// 查询每日登录数
	var loginCounts []dayCount
	if err := db.Table("admin_login_logs").
		Select(dateExpr+" AS date, COUNT(*) AS count").
		Where("deleted_at = 0 AND login_status = ? AND uid > 0 AND created_at >= ? AND created_at < ?", model.LoginStatusSuccess, sevenDaysAgo, todayStart.AddDate(0, 0, 1)).
		Group(dateExpr).
		Order("date ASC").
		Scan(&loginCounts).Error; err != nil {
		return nil, err
	}

	loginMap := make(map[string]int64, len(loginCounts))
	for _, c := range loginCounts {
		loginMap[c.Date] = c.Count
	}

	requests := make([]int64, 7)
	errors := make([]int64, 7)
	logins := make([]int64, 7)
	for i := 0; i < 7; i++ {
		dateKey := sevenDaysAgo.AddDate(0, 0, i).Format("2006-01-02")
		requests[i] = requestMap[dateKey]
		errors[i] = errorMap[dateKey]
		logins[i] = loginMap[dateKey]
	}

	return &TrendData{
		Dates:    dates,
		Requests: requests,
		Errors:   errors,
		Logins:   logins,
	}, nil
}

func requestTrendFromStats(db *gorm.DB, start time.Time, end time.Time) (map[string]int64, map[string]int64, error) {
	type row struct {
		Date         string
		RequestCount int64
		ErrorCount   int64
	}
	dateExpr := dailyRequestStatsDateExpr(db)
	var rows []row
	if err := db.Table("daily_request_stats").
		Select(dateExpr+" AS date, COALESCE(SUM(total_requests), 0) AS request_count, COALESCE(SUM(error_requests), 0) AS error_count").
		Where("date >= ? AND date < ?", statsDateKey(start), statsDateKey(end)).
		Group(dateExpr).
		Order("date ASC").
		Scan(&rows).Error; err != nil {
		return nil, nil, err
	}

	requests := make(map[string]int64, len(rows))
	errors := make(map[string]int64, len(rows))
	for _, item := range rows {
		requests[item.Date] = item.RequestCount
		errors[item.Date] = item.ErrorCount
	}
	return requests, errors, nil
}

func requestLogDateExpr(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		panic("requestLogDateExpr: db or db.Dialector is nil")
	}

	switch db.Dialector.Name() {
	case "sqlite":
		return "strftime('%Y-%m-%d', created_at)"
	case "mysql":
		// DSN 启用 parseTime=true 后，DATE() 会被驱动转为 time.Time，Scan 到 string 字段时会被
		// database/sql 用 RFC3339Nano 序列化为 "2006-01-02T00:00:00+08:00"，无法匹配下方按
		// "2006-01-02" 构造的 map key。MySQL 下使用 DATE_FORMAT 直接返回字符串规避此行为。
		return "DATE_FORMAT(created_at, '%Y-%m-%d')"
	default:
		panic(fmt.Sprintf("requestLogDateExpr: unsupported database driver: %s", db.Dialector.Name()))
	}
}

func dailyRequestStatsDateExpr(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		panic("dailyRequestStatsDateExpr: db or db.Dialector is nil")
	}

	switch db.Dialector.Name() {
	case "sqlite":
		return "strftime('%Y-%m-%d', date)"
	case "mysql":
		return "DATE_FORMAT(date, '%Y-%m-%d')"
	default:
		panic(fmt.Sprintf("dailyRequestStatsDateExpr: unsupported database driver: %s", db.Dialector.Name()))
	}
}

func statsDateKey(t time.Time) string {
	return t.Format("2006-01-02")
}

// responseTimeBucketDefs 定义耗时分桶边界，遵循左闭右开规则，最后一个桶为开区间。
// 边界归属：100ms 落入 100_300ms；300ms 落入 300_1000ms；1000ms 落入 1s_3s；3000ms 落入 gt_3s。
var responseTimeBucketDefs = []struct {
	Label   string
	LowerMs int
	UpperMs int // 0 表示无上界
}{
	{Label: "lt_100ms", LowerMs: 0, UpperMs: 100},
	{Label: "100_300ms", LowerMs: 100, UpperMs: 300},
	{Label: "300_1000ms", LowerMs: 300, UpperMs: 1000},
	{Label: "1s_3s", LowerMs: 1000, UpperMs: 3000},
	{Label: "gt_3s", LowerMs: 3000, UpperMs: 0},
}

// buildResponseTimeStats 聚合 daily_request_stats 的请求耗时分桶与平均耗时（输出毫秒）。
func buildResponseTimeStats(db *gorm.DB, start time.Time, end time.Time) *ResponseTimeStats {
	if db == nil {
		return nil
	}

	type row struct {
		TotalRequests       int64
		DurationLt100ms     int64
		Duration100To300ms  int64
		Duration300To1000ms int64
		Duration1sTo3s      int64
		DurationGt3s        int64
		TotalDurationMs     int64
	}
	var stat row
	if err := db.Table("daily_request_stats").
		Select(`
			COALESCE(SUM(total_requests), 0) AS total_requests,
			COALESCE(SUM(duration_lt_100ms), 0) AS duration_lt100ms,
			COALESCE(SUM(duration_100_300ms), 0) AS duration100_to300ms,
			COALESCE(SUM(duration_300_1000ms), 0) AS duration300_to1000ms,
			COALESCE(SUM(duration_1s_3s), 0) AS duration1s_to3s,
			COALESCE(SUM(duration_gt_3s), 0) AS duration_gt3s,
			COALESCE(SUM(total_duration_ms), 0) AS total_duration_ms
		`).
		Where("date >= ? AND date < ?", statsDateKey(start), statsDateKey(end)).
		Scan(&stat).Error; err != nil {
		if log.Logger != nil {
			log.Logger.Warn("统计请求耗时分桶失败", zap.Error(err))
		}
		return nil
	}

	counts := []int64{
		stat.DurationLt100ms,
		stat.Duration100To300ms,
		stat.Duration300To1000ms,
		stat.Duration1sTo3s,
		stat.DurationGt3s,
	}
	stats := &ResponseTimeStats{
		Buckets: make([]ResponseTimeBucket, 0, len(responseTimeBucketDefs)),
	}
	for i, def := range responseTimeBucketDefs {
		stats.Buckets = append(stats.Buckets, ResponseTimeBucket{
			Label:   def.Label,
			LowerMs: def.LowerMs,
			UpperMs: def.UpperMs,
			Count:   counts[i],
		})
	}
	if stat.TotalRequests > 0 {
		avgMs := float64(stat.TotalDurationMs) / float64(stat.TotalRequests)
		stats.AvgMs = math.Round(avgMs*100) / 100
	}
	return stats
}

// buildErrorCodeStats 聚合指定时间窗口内 HTTP 错误 response_status 分布，按数量降序取 TOP10。
func buildErrorCodeStats(db *gorm.DB, start time.Time, end time.Time, all ...bool) []ErrorCodeBucket {
	if db == nil {
		return nil
	}
	var rows []statusDistributionRow
	if err := db.Table("daily_request_stats").
		Select("http_status_distribution AS distribution").
		Where("date >= ? AND date < ?", statsDateKey(start), statsDateKey(end)).
		Scan(&rows).Error; err != nil {
		if log.Logger != nil {
			log.Logger.Warn("统计响应状态码分布失败", zap.Error(err))
		}
		return nil
	}
	return topErrorCodeBuckets(rows, true, all...)
}

type statusDistributionRow struct {
	Distribution string
}

func topErrorCodeBuckets(rows []statusDistributionRow, httpErrorsOnly bool, all ...bool) []ErrorCodeBucket {
	counts := map[int]int64{}
	for _, row := range rows {
		if row.Distribution == "" {
			continue
		}
		values := map[string]int64{}
		if err := json.Unmarshal([]byte(row.Distribution), &values); err != nil {
			continue
		}
		for key, count := range values {
			statusCode, err := strconv.Atoi(key)
			if err != nil {
				continue
			}
			if httpErrorsOnly && statusCode < 400 {
				continue
			}
			counts[statusCode] += count
		}
	}
	buckets := make([]ErrorCodeBucket, 0, len(counts))
	for statusCode, count := range counts {
		buckets = append(buckets, ErrorCodeBucket{StatusCode: statusCode, Count: count})
	}
	sortErrorCodeBuckets(buckets)
	if len(buckets) > 10 && (len(all) == 0 || !all[0]) {
		return buckets[:10]
	}
	return buckets
}

func sortErrorCodeBuckets(buckets []ErrorCodeBucket) {
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].Count == buckets[j].Count {
			return buckets[i].StatusCode < buckets[j].StatusCode
		}
		return buckets[i].Count > buckets[j].Count
	})
}

// buildStorageStats 统计 upload_files 表中存活文件的总量、总大小，并按 file_type 聚合。
func buildStorageStats(db *gorm.DB) *StorageStats {
	if db == nil {
		return nil
	}
	stats := &StorageStats{ByType: []StorageTypeBucket{}}

	var summary struct {
		TotalCount int64
		TotalSize  int64
	}
	if err := db.Table("upload_files").
		Select("COUNT(*) AS total_count, COALESCE(SUM(size), 0) AS total_size").
		Where("deleted_at = ?", 0).
		Scan(&summary).Error; err != nil {
		if log.Logger != nil {
			log.Logger.Warn("统计文件存储总量失败", zap.Error(err))
		}
		return nil
	}
	stats.TotalCount = summary.TotalCount
	stats.TotalSizeBytes = summary.TotalSize

	type row struct {
		FileType  string
		Count     int64
		SizeBytes int64
	}
	var rows []row
	if err := db.Table("upload_files").
		Select("file_type AS file_type, COUNT(*) AS count, COALESCE(SUM(size), 0) AS size_bytes").
		Where("deleted_at = ?", 0).
		Group("file_type").
		Order("size_bytes DESC, count DESC").
		Scan(&rows).Error; err != nil {
		if log.Logger != nil {
			log.Logger.Warn("统计文件存储分类失败", zap.Error(err))
		}
		return stats
	}
	for _, r := range rows {
		fileType := r.FileType
		if fileType == "" {
			fileType = "other"
		}
		stats.ByType = append(stats.ByType, StorageTypeBucket{
			FileType:  fileType,
			Count:     r.Count,
			SizeBytes: r.SizeBytes,
		})
	}
	return stats
}
