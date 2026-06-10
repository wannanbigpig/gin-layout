package dashboard

import (
	"fmt"
	"math"
	"strconv"
	"strings"
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
	Key   string `json:"key"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Time  string `json:"time"`
	Type  string `json:"type"`
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

	requestsToday, err := countRows(db.Table("request_logs").Where("created_at >= ? AND created_at < ?", todayStart, tomorrowStart))
	if err != nil {
		return nil, err
	}
	requestsYesterday, err := countRows(db.Table("request_logs").Where("created_at >= ? AND created_at < ?", yesterdayStart, todayStart))
	if err != nil {
		return nil, err
	}

	errorsToday, err := countRows(db.Table("request_logs").Where("created_at >= ? AND created_at < ? AND (operation_status <> 0 OR response_status >= 400)", todayStart, tomorrowStart))
	if err != nil {
		return nil, err
	}
	errorsYesterday, err := countRows(db.Table("request_logs").Where("created_at >= ? AND created_at < ? AND (operation_status <> 0 OR response_status >= 400)", yesterdayStart, todayStart))
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

	return &Overview{
		Metrics: []Metric{
			{Key: "users", Title: "活跃用户", Value: float64(activeUsers), Compare: "较昨日", Change: formatChange(activeUsers, activeUsersYesterday), Type: changeType(activeUsers, activeUsersYesterday)},
			{Key: "requests", Title: "请求总量", Value: float64(requestsToday), Compare: "较昨日", Change: formatChange(requestsToday, requestsYesterday), Type: changeType(requestsToday, requestsYesterday)},
			{Key: "errors", Title: "异常告警", Value: float64(errorsToday), Compare: "较昨日", Change: formatChange(errorsToday, errorsYesterday), Type: inverseChangeType(errorsToday, errorsYesterday)},
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

func countRows(db *gorm.DB) (int64, error) {
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
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
		OperatorAccount string
		OperationName   string
		Method          string
		BaseURL         string
		CreatedAt       time.Time
		OperationStatus int
	}
	var rows []row
	query := db.Table("request_logs").
		Select("id, operator_account, operation_name, method, base_url, created_at, operation_status")
	if operatorID > 0 {
		// 首页"最近动态"只展示当前登录用户自己的操作记录。
		query = query.Where("operator_id = ?", operatorID)
	}
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(4).
		Scan(&rows).Error; err != nil {
		log.Warn("查询最近动态失败", zap.Error(err))
	}

	activities := make([]ActivityItem, 0, len(rows))
	for _, item := range rows {
		title := item.OperationName
		if title == "" {
			title = item.Method + " " + item.BaseURL
		}
		activities = append(activities, ActivityItem{
			Key:   strconv.FormatUint(uint64(item.ID), 10),
			Title: title,
			Desc:  item.OperatorAccount,
			Time:  item.CreatedAt.Format("15:04"),
			Type:  activityType(item.OperationStatus),
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
	return prefix + strconvFormatFloat(change) + "%"
}

func strconvFormatFloat(value float64) string {
	return strconv.FormatFloat(math.Round(value*10)/10, 'f', 1, 64)
}

func changeType(current int64, previous int64) string {
	if current >= previous {
		return "success"
	}
	return "warning"
}

func inverseChangeType(current int64, previous int64) string {
	if current <= previous {
		return "success"
	}
	return "danger"
}

func activityType(status int) string {
	if status == 0 {
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

	dateExpr := requestLogDateExpr(db)

	// 查询每日请求数
	var requestCounts []dayCount
	if err := db.Table("request_logs").
		Select(dateExpr+" AS date, COUNT(*) AS count").
		Where("created_at >= ? AND created_at < ?", sevenDaysAgo, todayStart.AddDate(0, 0, 1)).
		Group(dateExpr).
		Order("date ASC").
		Scan(&requestCounts).Error; err != nil {
		return nil, err
	}

	// 查询每日错误数
	var errorCounts []dayCount
	if err := db.Table("request_logs").
		Select(dateExpr+" AS date, COUNT(*) AS count").
		Where("created_at >= ? AND created_at < ? AND (operation_status <> 0 OR response_status >= 400)", sevenDaysAgo, todayStart.AddDate(0, 0, 1)).
		Group(dateExpr).
		Order("date ASC").
		Scan(&errorCounts).Error; err != nil {
		return nil, err
	}

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

	// 将查询结果映射到日期
	requestMap := make(map[string]int64, len(requestCounts))
	for _, c := range requestCounts {
		requestMap[c.Date] = c.Count
	}
	errorMap := make(map[string]int64, len(errorCounts))
	for _, c := range errorCounts {
		errorMap[c.Date] = c.Count
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

// buildResponseTimeStats 聚合今日 request_logs.execution_time_us（微秒）的分桶分布与平均耗时（输出毫秒）。
//
// 单次扫描：用 SUM(CASE WHEN ...) 一并算出各分桶计数与 AVG，避免“每桶一条 COUNT + 一条 AVG”
// 造成对 request_logs 的多次全窗口扫描（原实现每次调用要扫 6 遍，dashboard 热路径开销翻倍）。
func buildResponseTimeStats(db *gorm.DB, start time.Time, end time.Time) *ResponseTimeStats {
	if db == nil {
		return nil
	}

	selectParts := make([]string, 0, len(responseTimeBucketDefs)+1)
	for i, def := range responseTimeBucketDefs {
		// LowerMs/UpperMs 均为内部定义的 int 常量，非外部输入，可安全内联。
		conds := make([]string, 0, 2)
		if def.LowerMs > 0 {
			conds = append(conds, fmt.Sprintf("execution_time_us >= %d", def.LowerMs*1000))
		}
		if def.UpperMs > 0 {
			conds = append(conds, fmt.Sprintf("execution_time_us < %d", def.UpperMs*1000))
		}
		when := "1=1"
		if len(conds) > 0 {
			when = strings.Join(conds, " AND ")
		}
		selectParts = append(selectParts, fmt.Sprintf("COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END), 0) AS b%d", when, i))
	}
	selectParts = append(selectParts, "COALESCE(AVG(execution_time_us), 0) / 1000.0 AS avg_ms")

	row := requestLogLatencyStatsQuery(db, start, end).
		Select(strings.Join(selectParts, ", ")).
		Row()

	counts := make([]int64, len(responseTimeBucketDefs))
	var avgMs float64
	dest := make([]any, 0, len(counts)+1)
	for i := range counts {
		dest = append(dest, &counts[i])
	}
	dest = append(dest, &avgMs)
	if err := row.Scan(dest...); err != nil {
		if log.Logger != nil {
			log.Logger.Warn("统计请求耗时分桶失败", zap.Error(err))
		}
		return nil
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
	stats.AvgMs = math.Round(avgMs*100) / 100
	return stats
}

func requestLogLatencyStatsQuery(db *gorm.DB, start time.Time, end time.Time) *gorm.DB {
	return db.Table("request_logs").
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("execution_time_scope = ?", model.ExecutionTimeScopeRequest)
}

// buildErrorCodeStats 聚合指定时间窗口内 HTTP 错误 response_status 分布，按数量降序取 TOP10。
func buildErrorCodeStats(db *gorm.DB, start time.Time, end time.Time) []ErrorCodeBucket {
	if db == nil {
		return nil
	}
	type row struct {
		StatusCode int
		Count      int64
	}
	var rows []row
	if err := db.Table("request_logs").
		Select("response_status AS status_code, COUNT(*) AS count").
		Where("created_at >= ? AND created_at < ? AND response_status >= ?", start, end, 400).
		Group("response_status").
		Order("count DESC, response_status ASC").
		Limit(10).
		Scan(&rows).Error; err != nil {
		if log.Logger != nil {
			log.Logger.Warn("统计响应状态码分布失败", zap.Error(err))
		}
		return nil
	}
	buckets := make([]ErrorCodeBucket, 0, len(rows))
	for _, r := range rows {
		buckets = append(buckets, ErrorCodeBucket(r))
	}
	return buckets
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
