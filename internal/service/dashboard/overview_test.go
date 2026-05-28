package dashboard

import (
	"testing"
	"time"

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
			base_url text,
			request_kind text,
			execution_time_scope text,
			response_status integer,
			operation_status integer,
			execution_time real
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
	if err := db.Exec(`INSERT INTO request_logs (created_at, updated_at, base_url, request_kind, execution_time_scope, response_status, operation_status, execution_time) VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
		createdAt, createdAt, baseURL, requestKind, executionScope, responseStatus, execMs).Error; err != nil {
		t.Fatalf("insert request_log failed: %v", err)
	}
}

func insertStreamRequestLog(t *testing.T, db *gorm.DB, createdAt time.Time, baseURL string, execMs float64, responseStatus int) {
	t.Helper()
	if err := db.Exec(`INSERT INTO request_logs (created_at, updated_at, base_url, request_kind, execution_time_scope, response_status, operation_status, execution_time) VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
		createdAt, createdAt, baseURL, "sse", "stream", responseStatus, execMs).Error; err != nil {
		t.Fatalf("insert sse request_log failed: %v", err)
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
	yesterday := todayStart.AddDate(0, 0, -1).Add(2 * time.Hour)

	// 落入 lt_100ms
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 50, 200)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 99.9, 200)
	// 边界：100ms 应进入 100_300ms
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 100, 200)
	// 边界：300ms 应进入 300_1000ms
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 300, 200)
	// 边界：1000ms 应进入 1s_3s
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 1000, 500)
	// 边界：3000ms 应进入 gt_3s
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 3000, 500)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 5000, 500)
	// 昨日数据不应计入今日
	insertRequestLog(t, db, yesterday, "", 50, 200)

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

func TestBuildResponseTimeStatsOnlyCountsRequestScope(t *testing.T) {
	db := newDashboardTestDB(t)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	insertRequestLog(t, db, todayStart.Add(time.Hour), "/admin/v1/dashboard/overview", 120, 200)
	insertRequestLog(t, db, todayStart.Add(time.Hour), notificationWSRequestPath, 85000, 200)
	insertStreamRequestLog(t, db, todayStart.Add(2*time.Hour), "/admin/v1/system/file/upload/credential/batch", 2200, 200)

	stats := buildResponseTimeStats(db, todayStart, tomorrowStart)
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.AvgMs != 120 {
		t.Fatalf("expected avg_ms=120 after excluding non-request scopes, got %v", stats.AvgMs)
	}

	expected := map[string]int64{
		"lt_100ms":   0,
		"100_300ms":  1,
		"300_1000ms": 0,
		"1s_3s":      0,
		"gt_3s":      0,
	}
	for _, bucket := range stats.Buckets {
		if bucket.Count != expected[bucket.Label] {
			t.Fatalf("bucket %s: expected count %d, got %d", bucket.Label, expected[bucket.Label], bucket.Count)
		}
	}
}

func TestBuildErrorCodeStatsOrderedDesc(t *testing.T) {
	db := newDashboardTestDB(t)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	// 500 出现 3 次，404 出现 2 次，403 出现 1 次，200/302 也应参与全量响应状态码分布。
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 500)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 500)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 500)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 404)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 404)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 403)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 200)
	insertRequestLog(t, db, todayStart.Add(time.Hour), "", 10, 302)

	buckets := buildErrorCodeStats(db, todayStart, tomorrowStart)
	if len(buckets) != 5 {
		t.Fatalf("expected 5 buckets, got %d", len(buckets))
	}
	if buckets[0].StatusCode != 500 || buckets[0].Count != 3 {
		t.Fatalf("bucket[0] expected 500/3, got %+v", buckets[0])
	}
	if buckets[1].StatusCode != 404 || buckets[1].Count != 2 {
		t.Fatalf("bucket[1] expected 404/2, got %+v", buckets[1])
	}
	if buckets[2].StatusCode != 200 || buckets[2].Count != 1 {
		t.Fatalf("bucket[2] expected 200/1, got %+v", buckets[2])
	}
	if buckets[3].StatusCode != 302 || buckets[3].Count != 1 {
		t.Fatalf("bucket[3] expected 302/1, got %+v", buckets[3])
	}
	if buckets[4].StatusCode != 403 || buckets[4].Count != 1 {
		t.Fatalf("bucket[4] expected 403/1, got %+v", buckets[4])
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

	insertRequestLog(t, db, todayStart.Add(-2*24*time.Hour).Add(time.Hour), "", 120, 200)
	insertRequestLog(t, db, todayStart.Add(-time.Hour), "", 220, 500)
	insertRequestLog(t, db, todayStart.Add(-time.Hour), "", 360, 404)

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
