package dashboard

import (
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

const statisticsWindowDays = 7
const statisticsMaxWindowDays = 90

type Statistics struct {
	WindowDays   int                `json:"window_days"`
	WindowStart  string             `json:"window_start"`
	WindowEnd    string             `json:"window_end"`
	GeneratedAt  string             `json:"generated_at"`
	Summary      StatisticsSummary  `json:"summary"`
	Trend        StatisticsTrend    `json:"trend"`
	ResponseTime *ResponseTimeStats `json:"response_time,omitempty"`
	Errors       ErrorDistribution  `json:"errors"`
	Storage      *StorageStats      `json:"storage,omitempty"`
}

type StatisticsSummary struct {
	TotalRequests         int64   `json:"total_requests"`
	TotalErrors           int64   `json:"total_errors"`
	TotalLogins           int64   `json:"total_logins"`
	AvgResponseMs         float64 `json:"avg_response_ms"`
	StorageTotalCount     int64   `json:"storage_total_count"`
	StorageTotalSizeBytes int64   `json:"storage_total_size_bytes"`
}

type StatisticsTrend struct {
	Days []StatisticsTrendDay `json:"days"`
}

type StatisticsTrendDay struct {
	Date         string `json:"date"`
	RequestCount int64  `json:"request_count"`
	ErrorCount   int64  `json:"error_count"`
	LoginCount   int64  `json:"login_count"`
}

type ErrorDistribution struct {
	Total          int64                 `json:"total"`
	Categories     []ErrorCategoryBucket `json:"categories"`
	StatusCode     []ErrorCodeBucket     `json:"status_codes"`
	BizStatusCodes []ErrorCodeBucket     `json:"biz_status_codes"`
}

type ErrorCategoryBucket struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Count int64  `json:"count"`
}

func (s *OverviewService) Statistics() (*Statistics, error) {
	return s.StatisticsRange("", "")
}

// StatisticsRange returns statistics for an inclusive calendar-date range.
// Empty dates preserve the original seven-day default contract.
func (s *OverviewService) StatisticsRange(startDate string, endDate string) (*Statistics, error) {
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	start, end, err := normalizeStatisticsRange(startDate, endDate, now)
	if err != nil {
		return nil, err
	}
	return buildStatisticsRange(db, now, start, end), nil
}

func buildStatistics(db *gorm.DB, now time.Time) *Statistics {
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return buildStatisticsRange(db, now, todayStart.AddDate(0, 0, -(statisticsWindowDays-1)), todayStart)
}

func buildStatisticsRange(db *gorm.DB, now time.Time, windowStart time.Time, inclusiveEnd time.Time) *Statistics {
	windowEnd := inclusiveEnd.AddDate(0, 0, 1)
	windowDays := calendarDays(windowStart, windowEnd)

	trendData, _ := buildTrendDataRange(db, windowStart, windowEnd)
	trend := StatisticsTrend{Days: make([]StatisticsTrendDay, 0, windowDays)}
	for i := 0; i < windowDays; i++ {
		day := StatisticsTrendDay{}
		if trendData != nil {
			day.Date = trendData.Dates[i]
			day.RequestCount = trendData.Requests[i]
			day.ErrorCount = trendData.Errors[i]
			day.LoginCount = trendData.Logins[i]
		} else {
			day.Date = windowStart.AddDate(0, 0, i).Format("01-02")
		}
		trend.Days = append(trend.Days, day)
	}

	responseTime := buildResponseTimeStats(db, windowStart, windowEnd)
	errors := buildErrorDistribution(db, windowStart, windowEnd)
	storage := buildStorageStats(db)

	summary := StatisticsSummary{}
	for _, day := range trend.Days {
		summary.TotalRequests += day.RequestCount
		summary.TotalErrors += day.ErrorCount
		summary.TotalLogins += day.LoginCount
	}
	if responseTime != nil {
		summary.AvgResponseMs = responseTime.AvgMs
	}
	if storage != nil {
		summary.StorageTotalCount = storage.TotalCount
		summary.StorageTotalSizeBytes = storage.TotalSizeBytes
	}

	return &Statistics{
		WindowDays:   windowDays,
		WindowStart:  windowStart.Format("2006-01-02"),
		WindowEnd:    windowEnd.Add(-time.Nanosecond).Format(time.RFC3339),
		GeneratedAt:  now.Format(time.RFC3339),
		Summary:      summary,
		Trend:        trend,
		ResponseTime: responseTime,
		Errors:       errors,
		Storage:      storage,
	}
}

func normalizeStatisticsRange(startDate string, endDate string, now time.Time) (time.Time, time.Time, error) {
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if startDate == "" && endDate == "" {
		return today.AddDate(0, 0, -(statisticsWindowDays - 1)), today, nil
	}
	if startDate == "" || endDate == "" {
		return time.Time{}, time.Time{}, e.NewBusinessError(e.InvalidParameter, "开始日期和结束日期必须同时提供")
	}
	start, startErr := time.ParseInLocation("2006-01-02", startDate, now.Location())
	end, endErr := time.ParseInLocation("2006-01-02", endDate, now.Location())
	if startErr != nil || endErr != nil {
		return time.Time{}, time.Time{}, e.NewBusinessError(e.InvalidParameter, "统计日期格式必须为 YYYY-MM-DD")
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, e.NewBusinessError(e.InvalidParameter, "统计结束日期不能早于开始日期")
	}
	if end.After(start.AddDate(0, 0, statisticsMaxWindowDays-1)) {
		return time.Time{}, time.Time{}, e.NewBusinessError(e.InvalidParameter, "统计时间范围不能超过90天")
	}
	return start, end, nil
}

func calendarDays(start time.Time, exclusiveEnd time.Time) int {
	days := 0
	for current := start; current.Before(exclusiveEnd); current = current.AddDate(0, 0, 1) {
		days++
	}
	return days
}

func buildErrorDistribution(db *gorm.DB, start time.Time, end time.Time) ErrorDistribution {
	distribution := ErrorDistribution{
		Categories: []ErrorCategoryBucket{
			{Key: "http_4xx", Title: "HTTP 4xx"},
			{Key: "http_5xx", Title: "HTTP 5xx"},
			{Key: "biz_failure", Title: "业务失败"},
		},
		StatusCode:     buildErrorCodeStats(db, start, end, true),
		BizStatusCodes: buildBizStatusCodeStats(db, start, end, true),
	}
	if db == nil {
		return distribution
	}

	type categoryCount struct {
		ErrorRequests int64
	}
	var counts categoryCount
	if err := db.Table("daily_request_stats").
		Select("COALESCE(SUM(error_requests), 0) AS error_requests").
		Where("date >= ? AND date < ?", statsDateKey(start), statsDateKey(end)).
		Scan(&counts).Error; err != nil {
		logger.Warn("query daily_request_stats error distribution failed", zap.Error(err))
	}

	for _, bucket := range distribution.StatusCode {
		if bucket.StatusCode >= 400 && bucket.StatusCode < 500 {
			distribution.Categories[0].Count += bucket.Count
		}
		if bucket.StatusCode >= 500 {
			distribution.Categories[1].Count += bucket.Count
		}
	}
	bizFailureTotal := int64(0)
	for _, bucket := range distribution.BizStatusCodes {
		bizFailureTotal += bucket.Count
	}
	distribution.Categories[2].Count = bizFailureTotal
	distribution.Total = counts.ErrorRequests
	// 类别总量先按完整分布计算，展示列表仍只保留 Top10。
	if len(distribution.StatusCode) > 10 {
		distribution.StatusCode = distribution.StatusCode[:10]
	}
	if len(distribution.BizStatusCodes) > 10 {
		distribution.BizStatusCodes = distribution.BizStatusCodes[:10]
	}

	return distribution
}

func buildBizStatusCodeStats(db *gorm.DB, start time.Time, end time.Time, all ...bool) []ErrorCodeBucket {
	if db == nil {
		return nil
	}
	var rows []statusDistributionRow
	if err := db.Table("daily_request_stats").
		Select("biz_status_distribution AS distribution").
		Where("date >= ? AND date < ?", statsDateKey(start), statsDateKey(end)).
		Scan(&rows).Error; err != nil {
		return nil
	}
	return topErrorCodeBuckets(rows, false, all...)
}
