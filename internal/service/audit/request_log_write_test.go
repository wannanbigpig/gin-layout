package audit

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestPersistAuditLogsBatchHandlesEmptyAndNil 验证批量落库在边界输入下不会触发 DB 调用。
//
// 设计要点：
//   - 当 snapshots 为空、全部 nil 或缺少 RequestID 时，函数应直接返回 nil；
//     这避免了在 DB 未初始化的测试环境里因调用 model.GetDB() 而失败。
//   - DB 集成路径由 middleware 层通过依赖注入的 PersistBatch 测试覆盖，本处仅做边界校验。
func TestPersistAuditLogsBatchHandlesEmptyAndNil(t *testing.T) {
	if err := PersistAuditLogsBatch(context.Background(), nil); err != nil {
		t.Fatalf("nil input should not error, got %v", err)
	}
	if err := PersistAuditLogsBatch(context.Background(), []*AuditLogSnapshot{}); err != nil {
		t.Fatalf("empty input should not error, got %v", err)
	}
	if err := PersistAuditLogsBatch(context.Background(), []*AuditLogSnapshot{nil, nil}); err != nil {
		t.Fatalf("all-nil input should not error, got %v", err)
	}
	// 缺少 RequestID 视为无效快照，应被过滤后短路。
	if err := PersistAuditLogsBatch(context.Background(), []*AuditLogSnapshot{{Method: "GET"}}); err != nil {
		t.Fatalf("snapshots without request id should be skipped, got %v", err)
	}
}

func TestPersistAuditLogWithDBIsIdempotentByRequestID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE request_logs (
		id integer primary key autoincrement,
		created_at datetime, updated_at datetime,
		request_id text not null unique, jwt_id text, operator_id integer,
		ip text, user_agent text, os text, browser text, request_kind text,
		method text, base_url text, operation_name text, operation_status integer,
		is_high_risk integer, operator_account text, operator_name text,
		request_headers text, request_query text, request_body text, change_diff text,
		response_status integer, response_body text, response_header text,
		execution_time_us integer, execution_time_scope text
	)`).Error; err != nil {
		t.Fatalf("create request logs: %v", err)
	}

	ctx := context.Background()
	if err := persistAuditLogWithDB(ctx, db, &AuditLogSnapshot{RequestID: "req-idempotent", OperationName: "first", ResponseStatus: 200}); err != nil {
		t.Fatalf("persist first snapshot: %v", err)
	}
	if err := persistAuditLogWithDB(ctx, db, &AuditLogSnapshot{RequestID: "req-idempotent", OperationName: "final", ResponseStatus: 500}); err != nil {
		t.Fatalf("persist retried snapshot: %v", err)
	}

	var rows []model.RequestLogs
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("query request logs: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("same request must produce one audit row, got %d", len(rows))
	}
	if rows[0].OperationName != "final" || rows[0].ResponseStatus != 500 {
		t.Fatalf("retry should refresh the same audit row, got %#v", rows[0])
	}
}

func TestSnapshotToRequestLogNormalizesDatabaseBoundaries(t *testing.T) {
	row := snapshotToRequestLog(&AuditLogSnapshot{
		RequestID:          strings.Repeat("请", 80),
		JwtID:              strings.Repeat("j", 60),
		UserAgent:          strings.Repeat("览", 1100),
		BaseURL:            "/admin/v1/file/" + strings.Repeat("长", 200),
		OperationName:      strings.Repeat("操", 300),
		RequestHeaders:     strings.Repeat("头", requestLogTextMaxBytes),
		ExecutionTimeScope: strings.Repeat("scope", 8),
	})

	if got := utf8.RuneCountInString(row.RequestID); got != 64 {
		t.Fatalf("request_id rune count = %d, want 64", got)
	}
	if got := utf8.RuneCountInString(row.JwtID); got != 36 {
		t.Fatalf("jwt_id rune count = %d, want 36", got)
	}
	if got := utf8.RuneCountInString(row.UserAgent); got != 1024 {
		t.Fatalf("user_agent rune count = %d, want 1024", got)
	}
	if got := utf8.RuneCountInString(row.BaseURL); got != 160 {
		t.Fatalf("base_url rune count = %d, want 160", got)
	}
	if got := utf8.RuneCountInString(row.OperationName); got != 255 {
		t.Fatalf("operation_name rune count = %d, want 255", got)
	}
	if got := len(row.RequestHeaders); got > requestLogTextMaxBytes {
		t.Fatalf("request_headers bytes = %d, want <= %d", got, requestLogTextMaxBytes)
	}
	if !utf8.ValidString(row.RequestHeaders) || !strings.HasSuffix(row.RequestHeaders, truncatedAuditSuffix) {
		t.Fatalf("request_headers should remain valid UTF-8 and carry truncation marker")
	}
	if got := utf8.RuneCountInString(row.ExecutionTimeScope); got != 16 {
		t.Fatalf("execution_time_scope rune count = %d, want 16", got)
	}

	invalid := snapshotToRequestLog(&AuditLogSnapshot{RequestID: "req-invalid", BaseURL: "bad\xffpath"})
	if !utf8.ValidString(invalid.BaseURL) {
		t.Fatal("invalid UTF-8 should be normalized before persistence")
	}
}

// TestSnapshotToRequestLogMapsAllFields 验证字段映射完整。
//
// 该测试保护后续新增字段时不会被遗忘：每个 snapshot 上的字段必须出现在落库行上。
func TestSnapshotToRequestLogMapsAllFields(t *testing.T) {
	snap := &AuditLogSnapshot{
		OccurredAt:         time.Date(2026, 9, 17, 1, 2, 3, 0, time.Local),
		RequestID:          "req-1",
		JwtID:              "jwt-1",
		OperatorID:         9,
		OperatorAccount:    "acc",
		OperatorName:       "name",
		IP:                 "127.0.0.1",
		UserAgent:          "ua",
		OS:                 "mac",
		Browser:            "chrome",
		RequestKind:        "http",
		Method:             "POST",
		BaseURL:            "/api",
		OperationName:      "op",
		OperationStatus:    0,
		IsHighRisk:         1,
		RequestHeaders:     "{h}",
		RequestQuery:       "q=1",
		RequestBody:        "{b}",
		ChangeDiff:         "{d}",
		ResponseStatus:     200,
		ResponseBody:       "{r}",
		ResponseHeader:     "{rh}",
		ExecutionTimeUs:    12340,
		ExecutionTimeScope: "request",
	}
	row := snapshotToRequestLog(snap)
	if !row.CreatedAt.Time.Equal(snap.OccurredAt) {
		t.Fatalf("occurred_at should be preserved as created_at: got=%v want=%v", row.CreatedAt.Time, snap.OccurredAt)
	}
	if row.RequestID != snap.RequestID || row.JwtID != snap.JwtID || row.OperatorID != snap.OperatorID {
		t.Fatalf("identity fields mismatch: %#v", row)
	}
	if row.OperatorAccount != snap.OperatorAccount || row.OperatorName != snap.OperatorName {
		t.Fatalf("operator fields mismatch: %#v", row)
	}
	if row.IP != snap.IP || row.UserAgent != snap.UserAgent || row.OS != snap.OS || row.Browser != snap.Browser {
		t.Fatalf("client fields mismatch: %#v", row)
	}
	if row.RequestKind != snap.RequestKind || row.Method != snap.Method || row.BaseURL != snap.BaseURL {
		t.Fatalf("routing fields mismatch: %#v", row)
	}
	if row.OperationName != snap.OperationName || row.OperationStatus != snap.OperationStatus || row.IsHighRisk != snap.IsHighRisk {
		t.Fatalf("operation fields mismatch: %#v", row)
	}
	if row.RequestHeaders != snap.RequestHeaders || row.RequestQuery != snap.RequestQuery || row.RequestBody != snap.RequestBody || row.ChangeDiff != snap.ChangeDiff {
		t.Fatalf("request payload fields mismatch: %#v", row)
	}
	if row.ResponseStatus != snap.ResponseStatus || row.ResponseBody != snap.ResponseBody || row.ResponseHeader != snap.ResponseHeader {
		t.Fatalf("response payload fields mismatch: %#v", row)
	}
	if row.ExecutionTimeUs != snap.ExecutionTimeUs || row.ExecutionTimeScope != snap.ExecutionTimeScope {
		t.Fatalf("execution time fields mismatch: %#v", row)
	}
}
