package dashboard

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

const statisticsWindowDays = 7

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
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	return buildStatistics(db, time.Now()), nil
}

func buildStatistics(db *gorm.DB, now time.Time) *Statistics {
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windowStart := todayStart.AddDate(0, 0, -(statisticsWindowDays - 1))
	windowEnd := todayStart.AddDate(0, 0, 1)

	trendData, _ := buildTrendData(db, todayStart)
	trend := StatisticsTrend{Days: make([]StatisticsTrendDay, 0, statisticsWindowDays)}
	for i := 0; i < statisticsWindowDays; i++ {
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
		WindowDays:   statisticsWindowDays,
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

func buildErrorDistribution(db *gorm.DB, start time.Time, end time.Time) ErrorDistribution {
	distribution := ErrorDistribution{
		Categories: []ErrorCategoryBucket{
			{Key: "http_4xx", Title: "HTTP 4xx"},
			{Key: "http_5xx", Title: "HTTP 5xx"},
			{Key: "biz_failure", Title: "业务失败"},
		},
		StatusCode:     buildErrorCodeStats(db, start, end),
		BizStatusCodes: buildBizStatusCodeStats(db, start, end),
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

	httpErrorTotal := int64(0)
	for _, bucket := range distribution.StatusCode {
		httpErrorTotal += bucket.Count
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

	return distribution
}

func buildBizStatusCodeStats(db *gorm.DB, start time.Time, end time.Time) []ErrorCodeBucket {
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
	return topErrorCodeBuckets(rows, false)
}
