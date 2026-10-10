package dashboard

import (
	"fmt"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newDashboardTestDB 构造覆盖 dashboard 聚合依赖表的 sqlite in-memory 数据库。
func newDashboardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	statements := []string{
		`CREATE TABLE request_logs (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			request_id text,
			operator_id integer default 0,
			operator_account text,
			operator_name text,
			operation_name text,
			method text,
			base_url text,
			request_kind text,
			execution_time_scope text,
			response_status integer,
			operation_status integer,
			is_high_risk integer default 0,
			change_diff text,
			execution_time_us integer
		)`,
		`CREATE TABLE daily_request_stats (
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
		)`,
		`CREATE TABLE upload_files (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			deleted_at integer default 0,
			file_type text,
			size integer
		)`,
		`CREATE TABLE admin_login_logs (
			id integer primary key autoincrement,
			uid integer,
			login_status integer,
			created_at datetime,
			updated_at datetime,
			deleted_at integer default 0
		)`,
		`CREATE TABLE task_runs (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			task_code text,
			kind text,
			source text,
			source_id text,
			queue text,
			trigger_user_id integer,
			trigger_account text,
			status text,
			attempt integer,
			max_retry integer,
			retry_of_run_id integer,
			retry_root_run_id integer,
			retry_seq integer,
			payload text,
			error_message text,
			started_at datetime,
			finished_at datetime,
			duration_ms real
		)`,
		`CREATE TABLE task_run_stats (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			task_code text,
			kind text,
			source text,
			window_start datetime,
			window_size text,
			total_count integer default 0,
			success_count integer default 0,
			failed_count integer default 0,
			canceled_count integer default 0,
			timeout_count integer default 0,
			interrupted_count integer default 0,
			sampled_success_count integer default 0,
			duration_total_ms real default 0,
			duration_max_ms real default 0
		)`,
		`CREATE TABLE sys_messages (
			id integer primary key autoincrement,
			user_id integer,
			is_read integer default 0,
			deleted_at datetime,
			created_at datetime
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create table failed: %v", err)
		}
	}
	return db
}

func insertRequestLog(t *testing.T, db *gorm.DB, createdAt time.Time, baseURL string, execMs float64, responseStatus int) {
	t.Helper()
	if baseURL == "" {
		baseURL = "/admin/v1/dashboard/overview"
	}
	requestKind := "http"
	executionScope := "request"
	if baseURL == notificationWSRequestPath {
		requestKind = "websocket"
		executionScope = "connection"
	}
	if err := db.Exec(`INSERT INTO request_logs (created_at, updated_at, base_url, request_kind, execution_time_scope, response_status, operation_status, execution_time_us) VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
		createdAt, createdAt, baseURL, requestKind, executionScope, responseStatus, int64(execMs*1000)).Error; err != nil {
		t.Fatalf("insert request_log failed: %v", err)
	}
}

func insertDailyRequestStat(t *testing.T, db *gorm.DB, statDate time.Time, totalRequests int64, errorRequests int64) {
	t.Helper()
	if err := db.Exec(`INSERT INTO daily_request_stats (date, total_requests, error_requests, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		statsDateKey(statDate), totalRequests, errorRequests, statDate, statDate).Error; err != nil {
		t.Fatalf("insert daily_request_stats failed: %v", err)
	}
}

func insertDailyRequestStatFull(t *testing.T, db *gorm.DB, statDate time.Time, values map[string]any) {
	t.Helper()
	defaults := map[string]any{
		"date":                     statsDateKey(statDate),
		"total_requests":           int64(0),
		"error_requests":           int64(0),
		"duration_lt_100ms":        int64(0),
		"duration_100_300ms":       int64(0),
		"duration_300_1000ms":      int64(0),
		"duration_1s_3s":           int64(0),
		"duration_gt_3s":           int64(0),
		"total_duration_ms":        int64(0),
		"http_status_distribution": "{}",
		"biz_status_distribution":  "{}",
		"created_at":               statDate,
		"updated_at":               statDate,
	}
	for key, value := range values {
		defaults[key] = value
	}
	if err := db.Exec(`INSERT INTO daily_request_stats (
		date, total_requests, error_requests,
		duration_lt_100ms, duration_100_300ms, duration_300_1000ms, duration_1s_3s, duration_gt_3s,
		total_duration_ms, http_status_distribution, biz_status_distribution, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		defaults["date"],
		defaults["total_requests"],
		defaults["error_requests"],
		defaults["duration_lt_100ms"],
		defaults["duration_100_300ms"],
		defaults["duration_300_1000ms"],
		defaults["duration_1s_3s"],
		defaults["duration_gt_3s"],
		defaults["total_duration_ms"],
		defaults["http_status_distribution"],
		defaults["biz_status_distribution"],
		defaults["created_at"],
		defaults["updated_at"],
	).Error; err != nil {
		t.Fatalf("insert full daily_request_stats failed: %v", err)
	}
}

func insertUploadFile(t *testing.T, db *gorm.DB, fileType string, size int64, deleted int) {
	t.Helper()
	now := time.Now()
	if err := db.Exec(`INSERT INTO upload_files (created_at, updated_at, deleted_at, file_type, size) VALUES (?, ?, ?, ?, ?)`,
		now, now, deleted, fileType, size).Error; err != nil {
		t.Fatalf("insert upload_file failed: %v", err)
	}
}

func TestBuildResponseTimeStatsBucketBoundaries(t *testing.T) {
	db := newDashboardTestDB(t)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	insertDailyRequestStatFull(t, db, todayStart, map[string]any{
		"total_requests":           int64(7),
		"error_requests":           int64(3),
		"duration_lt_100ms":        int64(2),
		"duration_100_300ms":       int64(1),
		"duration_300_1000ms":      int64(1),
		"duration_1s_3s":           int64(1),
		"duration_gt_3s":           int64(2),
		"total_duration_ms":        int64(9549),
		"http_status_distribution": `{"200":4,"500":3}`,
	})
	insertDailyRequestStatFull(t, db, todayStart.AddDate(0, 0, -1), map[string]any{
		"total_requests":           int64(1),
		"duration_lt_100ms":        int64(1),
		"total_duration_ms":        int64(50),
		"http_status_distribution": `{"200":1}`,
	})

	stats := buildResponseTimeStats(db, todayStart, tomorrowStart)
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if len(stats.Buckets) != 5 {
		t.Fatalf("expected 5 buckets, got %d", len(stats.Buckets))
	}
	expected := map[string]int64{
		"lt_100ms":   2,
		"100_300ms":  1,
		"300_1000ms": 1,
		"1s_3s":      1,
		"gt_3s":      2,
	}
	for _, bucket := range stats.Buckets {
		want, ok := expected[bucket.Label]
		if !ok {
			t.Fatalf("unexpected bucket label: %s", bucket.Label)
		}
		if bucket.Count != want {
			t.Fatalf("bucket %s: expected count %d, got %d", bucket.Label, want, bucket.Count)
		}
	}
	// 平均 = (50 + 99.9 + 100 + 300 + 1000 + 3000 + 5000) / 7 ≈ 1364.27
	if stats.AvgMs <= 0 {
		t.Fatalf("expected positive avg_ms, got %v", stats.AvgMs)
	}
}

func TestBuildErrorCodeStatsOrderedDesc(t *testing.T) {
	db := newDashboardTestDB(t)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	// 500 出现 3 次，404 出现 2 次，403 出现 1 次，101/200/302 不应进入错误响应状态码分布。
	insertDailyRequestStatFull(t, db, todayStart, map[string]any{
		"total_requests":           int64(9),
		"error_requests":           int64(6),
		"http_status_distribution": `{"500":3,"404":2,"403":1,"101":1,"200":1,"302":1}`,
	})

	buckets := buildErrorCodeStats(db, todayStart, tomorrowStart)
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}
	if buckets[0].StatusCode != 500 || buckets[0].Count != 3 {
		t.Fatalf("bucket[0] expected 500/3, got %+v", buckets[0])
	}
	if buckets[1].StatusCode != 404 || buckets[1].Count != 2 {
		t.Fatalf("bucket[1] expected 404/2, got %+v", buckets[1])
	}
	if buckets[2].StatusCode != 403 || buckets[2].Count != 1 {
		t.Fatalf("bucket[2] expected 403/1, got %+v", buckets[2])
	}
}

func TestBuildStorageStatsAggregates(t *testing.T) {
	db := newDashboardTestDB(t)

	insertUploadFile(t, db, "image", 1000, 0)
	insertUploadFile(t, db, "image", 2000, 0)
	insertUploadFile(t, db, "pdf", 5000, 0)
	insertUploadFile(t, db, "other", 200, 0)
	// 已删除的不计
	insertUploadFile(t, db, "image", 9999, 1)

	stats := buildStorageStats(db)
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.TotalCount != 4 {
		t.Fatalf("expected total_count=4, got %d", stats.TotalCount)
	}
	if stats.TotalSizeBytes != 8200 {
		t.Fatalf("expected total_size_bytes=8200, got %d", stats.TotalSizeBytes)
	}
	if len(stats.ByType) != 3 {
		t.Fatalf("expected 3 by_type entries, got %d", len(stats.ByType))
	}
	// 排序：size_bytes 降序，pdf(5000) > image(3000) > other(200)
	if stats.ByType[0].FileType != "pdf" || stats.ByType[0].SizeBytes != 5000 {
		t.Fatalf("by_type[0] expected pdf/5000, got %+v", stats.ByType[0])
	}
	if stats.ByType[1].FileType != "image" || stats.ByType[1].Count != 2 || stats.ByType[1].SizeBytes != 3000 {
		t.Fatalf("by_type[1] expected image/2/3000, got %+v", stats.ByType[1])
	}
	if stats.ByType[2].FileType != "other" {
		t.Fatalf("by_type[2] expected other, got %+v", stats.ByType[2])
	}
}

func TestBuildActivitiesReturnsMeaningfulStructuredOperations(t *testing.T) {
	db := newDashboardTestDB(t)
	now := time.Now().Truncate(time.Second)
	insertActivity := func(operatorID uint, method string, status int, highRisk uint8, changeDiff string, name string, createdAt time.Time) {
		t.Helper()
		if err := db.Exec(`INSERT INTO request_logs (
			created_at, updated_at, request_id, operator_id, operator_account, operator_name,
			operation_name, method, base_url, request_kind, execution_time_scope,
			response_status, operation_status, is_high_risk, change_diff, execution_time_us
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'http', 'request', 200, ?, ?, ?, 1000)`,
			createdAt, createdAt, "request-activity", operatorID, "admin", "管理员", name, method, "/admin/v1/resource", status, highRisk, changeDiff).Error; err != nil {
			t.Fatalf("insert activity failed: %v", err)
		}
	}

	insertActivity(7, "GET", 0, 0, "", "浏览列表", now.Add(-time.Minute))
	insertActivity(7, "POST", 0, 0, "", "创建资源", now.Add(-4*time.Minute))
	insertActivity(7, "GET", 0, 0, `{"name":{"before":"旧","after":"新"}}`, "数据变更", now.Add(-3*time.Minute))
	insertActivity(7, "GET", 500, 0, "", "读取失败", now.Add(-2*time.Minute))
	insertActivity(7, "GET", 0, 1, "", "查看敏感配置", now.Add(-time.Minute))
	insertActivity(8, "DELETE", 0, 1, "", "其他用户删除", now)
	for index := 0; index < 8; index++ {
		createdAt := now.Add(time.Duration(index) * time.Second)
		if err := db.Exec(`INSERT INTO request_logs (
			created_at, updated_at, request_id, operator_id, operator_account, operator_name,
			operation_name, method, base_url, request_kind, execution_time_scope,
			response_status, operation_status, is_high_risk, change_diff, execution_time_us
		) VALUES (?, ?, ?, 7, 'admin', '管理员', '生成 WebSocket 临时票据', 'POST', '/admin/v1/common/ws-ticket', 'http', 'request', 200, 0, 1, '', 1000)`,
			createdAt, createdAt, fmt.Sprintf("ws-ticket-%d", index)).Error; err != nil {
			t.Fatalf("insert websocket ticket activity failed: %v", err)
		}
	}

	activities := buildActivities(db, 7)
	if len(activities) != 4 {
		t.Fatalf("expected 4 meaningful activities, got %d: %+v", len(activities), activities)
	}
	if activities[0].OperationName != "查看敏感配置" || activities[0].IsHighRisk != 1 {
		t.Fatalf("unexpected latest activity: %+v", activities[0])
	}
	if activities[1].OperationStatus != 500 || activities[1].Type != "warning" {
		t.Fatalf("expected failed activity metadata, got %+v", activities[1])
	}
	if !activities[2].HasChangeDiff || activities[2].OperationName != "数据变更" {
		t.Fatalf("expected change-diff activity, got %+v", activities[2])
	}
	if activities[3].Method != "POST" || activities[3].RequestLogID == 0 || activities[3].RequestID != "request-activity" {
		t.Fatalf("expected structured write activity, got %+v", activities[3])
	}
	if !activities[0].CreatedAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("expected full created_at timestamp, got %v", activities[0].CreatedAt)
	}
}

func TestBuildStatsEmptyData(t *testing.T) {
	db := newDashboardTestDB(t)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	rt := buildResponseTimeStats(db, todayStart, tomorrowStart)
	if rt == nil {
		t.Fatal("expected non-nil ResponseTimeStats for empty data")
	}
	if len(rt.Buckets) != 5 {
		t.Fatalf("expected 5 buckets, got %d", len(rt.Buckets))
	}
	for _, b := range rt.Buckets {
		if b.Count != 0 {
			t.Fatalf("expected bucket count 0, got %d for %s", b.Count, b.Label)
		}
	}
	if rt.AvgMs != 0 {
		t.Fatalf("expected avg_ms=0 for empty data, got %v", rt.AvgMs)
	}

	errs := buildErrorCodeStats(db, todayStart, tomorrowStart)
	if len(errs) != 0 {
		t.Fatalf("expected 0 error buckets, got %d", len(errs))
	}

	storage := buildStorageStats(db)
	if storage == nil {
		t.Fatal("expected non-nil StorageStats for empty data")
	}
	if storage.TotalCount != 0 || storage.TotalSizeBytes != 0 {
		t.Fatalf("expected zero totals, got count=%d size=%d", storage.TotalCount, storage.TotalSizeBytes)
	}
	if len(storage.ByType) != 0 {
		t.Fatalf("expected empty by_type, got %d", len(storage.ByType))
	}
}

func TestResponseTimeBucketDefinitionsStable(t *testing.T) {
	// 桶顺序与 label 必须与前端契约一致，避免误改。
	wantLabels := []string{"lt_100ms", "100_300ms", "300_1000ms", "1s_3s", "gt_3s"}
	if len(responseTimeBucketDefs) != len(wantLabels) {
		t.Fatalf("expected %d bucket defs, got %d", len(wantLabels), len(responseTimeBucketDefs))
	}
	for i, def := range responseTimeBucketDefs {
		if def.Label != wantLabels[i] {
			t.Fatalf("bucket[%d] expected label %s, got %s", i, wantLabels[i], def.Label)
		}
	}
}

func TestBuildStatisticsAggregatesSummaryAndTrend(t *testing.T) {
	db := newDashboardTestDB(t)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	insertDailyRequestStatFull(t, db, todayStart.AddDate(0, 0, -2), map[string]any{
		"total_requests":           int64(1),
		"duration_100_300ms":       int64(1),
		"total_duration_ms":        int64(120),
		"http_status_distribution": `{"200":1}`,
		"biz_status_distribution":  `{}`,
	})
	insertDailyRequestStatFull(t, db, todayStart, map[string]any{
		"total_requests":           int64(2),
		"error_requests":           int64(2),
		"duration_100_300ms":       int64(1),
		"duration_300_1000ms":      int64(1),
		"total_duration_ms":        int64(580),
		"http_status_distribution": `{"500":1,"404":1}`,
		"biz_status_distribution":  `{"10001":1}`,
	})

	insertUploadFile(t, db, "image", 2048, 0)
	insertUploadFile(t, db, "pdf", 4096, 0)

	stats := buildStatistics(db, now)
	if stats == nil {
		t.Fatal("expected non-nil statistics")
	}
	if stats.WindowDays != statisticsWindowDays {
		t.Fatalf("unexpected window days: %d", stats.WindowDays)
	}
	if len(stats.Trend.Days) != statisticsWindowDays {
		t.Fatalf("expected %d trend days, got %d", statisticsWindowDays, len(stats.Trend.Days))
	}
	if stats.Summary.TotalRequests != 3 {
		t.Fatalf("expected total requests 3, got %d", stats.Summary.TotalRequests)
	}
	if stats.Summary.TotalErrors != 2 {
		t.Fatalf("expected total errors 2, got %d", stats.Summary.TotalErrors)
	}
	if stats.Summary.StorageTotalCount != 2 || stats.Summary.StorageTotalSizeBytes != 6144 {
		t.Fatalf("unexpected storage summary: %#v", stats.Summary)
	}
	if stats.ResponseTime == nil || len(stats.ResponseTime.Buckets) != 5 {
		t.Fatalf("unexpected response time stats: %#v", stats.ResponseTime)
	}
	if len(stats.Errors.Categories) != 3 {
		t.Fatalf("unexpected error categories: %#v", stats.Errors)
	}
	if stats.Storage == nil || stats.Storage.TotalCount != 2 {
		t.Fatalf("unexpected storage stats: %#v", stats.Storage)
	}
}

func TestBuildStatisticsSupportsThirtyDayAndCustomRanges(t *testing.T) {
	db := newDashboardTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	start := time.Date(2026, 8, 19, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	insertDailyRequestStat(t, db, start, 2, 0)
	insertDailyRequestStat(t, db, end, 3, 1)

	stats := buildStatisticsRange(db, now, start, end)
	if stats.WindowDays != 30 || len(stats.Trend.Days) != 30 {
		t.Fatalf("expected 30-day range, got window=%d points=%d", stats.WindowDays, len(stats.Trend.Days))
	}
	if stats.WindowStart != "2026-08-19" || stats.Summary.TotalRequests != 5 || stats.Summary.TotalErrors != 1 {
		t.Fatalf("unexpected custom statistics: %#v", stats)
	}
}

func TestNormalizeStatisticsRange(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	start, end, err := normalizeStatisticsRange("", "", now)
	if err != nil || start.Format("2006-01-02") != "2026-09-11" || end.Format("2006-01-02") != "2026-09-17" {
		t.Fatalf("unexpected default range: %s - %s, err=%v", start, end, err)
	}
	for _, dates := range [][2]string{
		{"2026-09-01", ""},
		{"2026-09-18", "2026-09-17"},
		{"2026-01-01", "2026-09-17"},
	} {
		if _, _, err := normalizeStatisticsRange(dates[0], dates[1], now); err == nil {
			t.Fatalf("expected invalid range %q - %q to fail", dates[0], dates[1])
		}
	}
}

func TestRequestStatsForRangeUsesDailyStatsBeforeRequestLogs(t *testing.T) {
	db := newDashboardTestDB(t)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	insertDailyRequestStat(t, db, todayStart, 20, 3)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 200)
	insertRequestLog(t, db, todayStart.Add(2*time.Hour), "", 10, 500)

	requests, errors, err := requestStatsForRange(db, todayStart, tomorrowStart)
	if err != nil {
		t.Fatalf("requestStatsForRange returned error: %v", err)
	}
	if requests != 20 || errors != 3 {
		t.Fatalf("expected stats table values 20/3, got %d/%d", requests, errors)
	}
}

func TestBuildTrendDataUsesDailyStatsOnly(t *testing.T) {
	db := newDashboardTestDB(t)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := todayStart.AddDate(0, 0, -1)

	insertDailyRequestStat(t, db, todayStart, 30, 4)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 200)
	insertRequestLog(t, db, todayStart.Add(2*time.Hour), "", 10, 500)
	insertRequestLog(t, db, yesterday.Add(time.Hour), "", 10, 200)
	insertRequestLog(t, db, yesterday.Add(2*time.Hour), "", 10, 500)

	trend, err := buildTrendData(db, todayStart)
	if err != nil {
		t.Fatalf("buildTrendData returned error: %v", err)
	}
	if trend.Requests[len(trend.Requests)-1] != 30 || trend.Errors[len(trend.Errors)-1] != 4 {
		t.Fatalf("expected today stats 30/4, got %d/%d", trend.Requests[len(trend.Requests)-1], trend.Errors[len(trend.Errors)-1])
	}
	if trend.Requests[len(trend.Requests)-2] != 0 || trend.Errors[len(trend.Errors)-2] != 0 {
		t.Fatalf("expected yesterday stats to remain 0 without daily_request_stats, got %d/%d", trend.Requests[len(trend.Requests)-2], trend.Errors[len(trend.Errors)-2])
	}
}

func insertTaskRun(t *testing.T, db *gorm.DB, createdAt time.Time, status string) {
	t.Helper()
	successCount := 0
	failedCount := 0
	if status == model.TaskRunStatusSuccess {
		successCount = 1
	} else {
		failedCount = 1
	}
	if err := db.Exec(`INSERT INTO task_run_stats (created_at, updated_at, task_code, kind, source, window_start, window_size, total_count, success_count, failed_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		createdAt, createdAt, "test_task", model.TaskKindAsync, model.TaskSourceQueue, createdAt, model.TaskRunStatsWindowHour, 1, successCount, failedCount).Error; err != nil {
		t.Fatalf("insert task_run_stat failed: %v", err)
	}
}

func TestTaskCompletionRateNoTasks(t *testing.T) {
	db := newDashboardTestDB(t)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	rate, err := taskCompletionRate(db, todayStart, tomorrowStart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 0 {
		t.Fatalf("expected 0%% when no tasks exist, got %v%%", rate)
	}
}

func TestTaskCompletionRatePartialCompletion(t *testing.T) {
	db := newDashboardTestDB(t)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	insertTaskRun(t, db, todayStart.Add(time.Hour), model.TaskRunStatusSuccess)
	insertTaskRun(t, db, todayStart.Add(time.Hour), model.TaskRunStatusSuccess)
	insertTaskRun(t, db, todayStart.Add(time.Hour), model.TaskRunStatusFailed)
	insertTaskRun(t, db, todayStart.Add(time.Hour), model.TaskRunStatusFailed)

	rate, err := taskCompletionRate(db, todayStart, tomorrowStart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 50 {
		t.Fatalf("expected 50%% completion rate, got %v%%", rate)
	}
}

func TestTaskCompletionRateFullCompletion(t *testing.T) {
	db := newDashboardTestDB(t)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	insertTaskRun(t, db, todayStart.Add(time.Hour), model.TaskRunStatusSuccess)
	insertTaskRun(t, db, todayStart.Add(time.Hour), model.TaskRunStatusSuccess)

	rate, err := taskCompletionRate(db, todayStart, tomorrowStart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 100 {
		t.Fatalf("expected 100%% completion rate, got %v%%", rate)
	}
}
