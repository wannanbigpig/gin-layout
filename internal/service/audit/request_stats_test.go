package audit

import (
	"net/http"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRequestStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE daily_request_stats (
		id integer primary key autoincrement,
		date date unique,
		total_requests integer default 0,
		error_requests integer default 0,
		duration_lt_100ms integer default 0,
		duration_100_300ms integer default 0,
		duration_300_1000ms integer default 0,
		duration_1s_3s integer default 0,
		duration_gt_3s integer default 0,
		total_duration_ms integer default 0,
		http_status_distribution text,
		biz_status_distribution text,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatalf("create daily_request_stats failed: %v", err)
	}
	return db
}

func TestRecordDailyRequestStatIncrementsCounters(t *testing.T) {
	db := newRequestStatsTestDB(t)
	now := time.Date(2026, 6, 12, 10, 30, 0, 0, time.Local)

	inputs := []*AuditLogSnapshot{
		{RequestID: "req-1", ResponseStatus: http.StatusOK, OperationStatus: 0, ExecutionTimeUs: 50_000},
		{RequestID: "req-2", ResponseStatus: http.StatusInternalServerError, OperationStatus: 0, ExecutionTimeUs: 150_000},
		{RequestID: "req-3", ResponseStatus: http.StatusOK, OperationStatus: 10001, ExecutionTimeUs: 3_500_000},
	}
	for _, input := range inputs {
		if err := recordDailyRequestStat(db, input, now); err != nil {
			t.Fatalf("recordDailyRequestStat returned error: %v", err)
		}
	}

	var got struct {
		TotalRequests int64
		ErrorRequests int64
		Lt100         int64
		Between100300 int64
		Gt3s          int64
		TotalDuration int64
	}
	if err := db.Table("daily_request_stats").Select(`
		total_requests,
		error_requests,
		duration_lt_100ms AS lt100,
		duration_100_300ms AS between100300,
		duration_gt_3s AS gt3s,
		total_duration_ms AS total_duration
	`).Scan(&got).Error; err != nil {
		t.Fatalf("scan stats failed: %v", err)
	}

	if got.TotalRequests != 3 || got.ErrorRequests != 2 {
		t.Fatalf("expected total/error 3/2, got %d/%d", got.TotalRequests, got.ErrorRequests)
	}
	if got.Lt100 != 1 || got.Between100300 != 1 || got.Gt3s != 1 {
		t.Fatalf("unexpected duration buckets: %#v", got)
	}
	if got.TotalDuration != 3700 {
		t.Fatalf("expected total duration 3700ms, got %d", got.TotalDuration)
	}
}
