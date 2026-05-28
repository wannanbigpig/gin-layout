package model

import (
	"time"
)

// DailyRequestStats 每日请求与错误统计表
type DailyRequestStats struct {
	ID                     uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Date                   time.Time `gorm:"column:date;type:date;uniqueIndex;not null" json:"date"`
	TotalRequests          uint64    `gorm:"column:total_requests;type:bigint;not null;default:0" json:"total_requests"`
	ErrorRequests          uint64    `gorm:"column:error_requests;type:bigint;not null;default:0" json:"error_requests"`
	DurationLt100ms        uint64    `gorm:"column:duration_lt_100ms;type:bigint;not null;default:0" json:"duration_lt_100ms"`
	Duration100To300ms     uint64    `gorm:"column:duration_100_300ms;type:bigint;not null;default:0" json:"duration_100_300ms"`
	Duration300To1000ms    uint64    `gorm:"column:duration_300_1000ms;type:bigint;not null;default:0" json:"duration_300_1000ms"`
	Duration1sTo3s         uint64    `gorm:"column:duration_1s_3s;type:bigint;not null;default:0" json:"duration_1s_3s"`
	DurationGt3s           uint64    `gorm:"column:duration_gt_3s;type:bigint;not null;default:0" json:"duration_gt_3s"`
	TotalDurationMs        uint64    `gorm:"column:total_duration_ms;type:bigint;not null;default:0" json:"total_duration_ms"`
	HttpStatusDistribution string    `gorm:"column:http_status_distribution;type:text" json:"http_status_distribution"`
	BizStatusDistribution  string    `gorm:"column:biz_status_distribution;type:text" json:"biz_status_distribution"`
	CreatedAt              time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 获取表名
func (DailyRequestStats) TableName() string {
	return "daily_request_stats"
}
