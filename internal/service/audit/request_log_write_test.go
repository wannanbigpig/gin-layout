package audit

import (
	"context"
	"testing"
)

// TestPersistAuditLogsBatchHandlesEmptyAndNil 验证批量落库在边界输入下不会触发 DB 调用。
//
// 设计要点：
// - 当 snapshots 为空、全部 nil 或缺少 RequestID 时，函数应直接返回 nil；
//   这避免了在 DB 未初始化的测试环境里因调用 model.GetDB() 而失败。
// - DB 集成路径由 middleware 层通过依赖注入的 PersistBatch 测试覆盖，本处仅做边界校验。
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

// TestSnapshotToRequestLogMapsAllFields 验证字段映射完整。
//
// 该测试保护后续新增字段时不会被遗忘：每个 snapshot 上的字段必须出现在落库行上。
func TestSnapshotToRequestLogMapsAllFields(t *testing.T) {
	snap := &AuditLogSnapshot{
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
		ExecutionTime:      12.34,
		ExecutionTimeUnit:  "ms",
		ExecutionTimeScope: "request",
	}
	row := snapshotToRequestLog(snap)
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
	if row.ExecutionTime != snap.ExecutionTime || row.ExecutionTimeUnit != snap.ExecutionTimeUnit || row.ExecutionTimeScope != snap.ExecutionTimeScope {
		t.Fatalf("execution time fields mismatch: %#v", row)
	}
}
