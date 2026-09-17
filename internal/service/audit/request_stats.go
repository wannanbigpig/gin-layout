package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

// RecordDailyRequestStat increments the real HTTP request counter before audit-log sampling.
func RecordDailyRequestStat(snapshot *AuditLogSnapshot) error {
	return RecordDailyRequestStatContext(context.Background(), snapshot)
}

// RecordDailyRequestStatContext 在调用方给定的时间预算内更新每日请求统计。
func RecordDailyRequestStatContext(ctx context.Context, snapshot *AuditLogSnapshot) error {
	if snapshot == nil || snapshot.RequestID == "" {
		return nil
	}
	db, err := model.GetDB()
	if err != nil {
		return err
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	return recordDailyRequestStat(db, snapshot, time.Now())
}

func recordDailyRequestStat(db *gorm.DB, snapshot *AuditLogSnapshot, now time.Time) error {
	if db == nil || snapshot == nil || snapshot.RequestID == "" {
		return nil
	}
	if snapshot.RequestKind != model.RequestKindHTTP {
		return nil
	}

	statDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	durationMs := uint64(0)
	if snapshot.ExecutionTimeUs > 0 {
		durationMs = uint64(snapshot.ExecutionTimeUs / 1000)
	}
	httpStatusDistribution := map[string]uint64{}
	bizStatusDistribution := map[string]uint64{}
	if snapshot.ResponseStatus > 0 {
		httpStatusDistribution[strconv.Itoa(snapshot.ResponseStatus)] = 1
	}
	if snapshot.OperationStatus != 0 {
		bizStatusDistribution[strconv.Itoa(snapshot.OperationStatus)] = 1
	}

	stat := &model.DailyRequestStats{
		Date:                   statDate,
		TotalRequests:          1,
		ErrorRequests:          errorRequestIncrement(snapshot),
		TotalDurationMs:        uint64(durationMs),
		HttpStatusDistribution: marshalCountMap(httpStatusDistribution),
		BizStatusDistribution:  marshalCountMap(bizStatusDistribution),
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	assignDurationBucket(stat, durationMs)

	updates := map[string]any{
		"total_requests":           gorm.Expr("total_requests + 1"),
		"error_requests":           gorm.Expr("error_requests + ?", stat.ErrorRequests),
		"duration_lt_100ms":        gorm.Expr("duration_lt_100ms + ?", stat.DurationLt100ms),
		"duration_100_300ms":       gorm.Expr("duration_100_300ms + ?", stat.Duration100To300ms),
		"duration_300_1000ms":      gorm.Expr("duration_300_1000ms + ?", stat.Duration300To1000ms),
		"duration_1s_3s":           gorm.Expr("duration_1s_3s + ?", stat.Duration1sTo3s),
		"duration_gt_3s":           gorm.Expr("duration_gt_3s + ?", stat.DurationGt3s),
		"total_duration_ms":        gorm.Expr("total_duration_ms + ?", stat.TotalDurationMs),
		"http_status_distribution": mergeDistributionExpr(db, "http_status_distribution", snapshot.ResponseStatus),
		"biz_status_distribution":  mergeDistributionExpr(db, "biz_status_distribution", snapshot.OperationStatus),
		"updated_at":               now,
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(stat).Error
}

func assignDurationBucket(stat *model.DailyRequestStats, durationMs uint64) {
	if stat == nil {
		return
	}
	switch {
	case durationMs < 100:
		stat.DurationLt100ms = 1
	case durationMs < 300:
		stat.Duration100To300ms = 1
	case durationMs < 1000:
		stat.Duration300To1000ms = 1
	case durationMs < 3000:
		stat.Duration1sTo3s = 1
	default:
		stat.DurationGt3s = 1
	}
}

func errorRequestIncrement(snapshot *AuditLogSnapshot) uint64 {
	if snapshot == nil {
		return 0
	}
	if snapshot.OperationStatus != 0 || snapshot.ResponseStatus >= http.StatusBadRequest {
		return 1
	}
	return 0
}

func marshalCountMap(values map[string]uint64) string {
	if len(values) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func mergeDistributionExpr(db *gorm.DB, column string, status int) any {
	if status == 0 {
		return gorm.Expr(column)
	}
	key := strconv.Itoa(status)
	switch {
	case db != nil && db.Dialector != nil && db.Dialector.Name() == "mysql":
		jsonPath := "$.\"" + key + "\""
		return gorm.Expr(
			"JSON_SET(COALESCE(NULLIF("+column+", ''), '{}'), ?, COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(COALESCE(NULLIF("+column+", ''), '{}'), ?)) AS UNSIGNED), 0) + 1)",
			jsonPath,
			jsonPath,
		)
	default:
		return gorm.Expr(column)
	}
}
