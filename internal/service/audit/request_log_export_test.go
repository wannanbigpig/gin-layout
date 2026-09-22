package audit

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestStreamExportCSVUsesKeysetPaginationWhenNewRowsInserted(t *testing.T) {
	db := newRequestLogExportTestDB(t)

	baseTime := time.Date(2026, 5, 23, 12, 0, 0, 0, time.Local)
	for i := 0; i < 501; i++ {
		createdAt := baseTime.Add(-time.Duration(i) * time.Second)
		if err := db.Exec(`INSERT INTO request_logs (
			id, request_id, operator_account, operator_name, ip, request_kind, execution_time_scope,
			method, base_url, operation_name, operation_status, is_high_risk, response_status,
			execution_time_us, change_diff, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			i+1, "req-"+strconv.Itoa(i+1), "admin", "管理员", "127.0.0.1", "http", "request",
			"GET", "/api/demo", "demo", 0, 0, 200, 10500, "[]", createdAt, createdAt,
		).Error; err != nil {
			t.Fatalf("insert request log %d failed: %v", i+1, err)
		}
	}

	restoreGetDB := requestLogGetDB
	requestLogGetDB = func(model ...any) (*gorm.DB, error) {
		if len(model) > 0 && model[0] != nil {
			return db.Model(model[0]), nil
		}
		return db, nil
	}
	defer func() { requestLogGetDB = restoreGetDB }()

	insertedNewest := false
	var output bytes.Buffer
	reporter := func(processed, total int) error {
		if processed == 500 && !insertedNewest {
			insertedNewest = true
			createdAt := baseTime.Add(time.Second)
			return db.Exec(`INSERT INTO request_logs (
				id, request_id, operator_account, operator_name, ip, request_kind, execution_time_scope,
				method, base_url, operation_name, operation_status, is_high_risk, response_status,
				execution_time_us, change_diff, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				9999, "req-newest", "admin", "管理员", "127.0.0.1", "http", "request",
				"GET", "/api/demo", "demo", 0, 0, 200, 10500, "[]", createdAt, createdAt,
			).Error
		}
		return nil
	}

	fileName, err := StreamExportCSV(context.Background(), &form.RequestLogExport{Limit: 1000}, &output, reporter)
	if err != nil {
		t.Fatalf("StreamExportCSV failed: %v", err)
	}
	if fileName == "" {
		t.Fatal("expected export file name")
	}

	rows, err := csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv failed: %v", err)
	}
	if len(rows) != 502 {
		t.Fatalf("unexpected csv row count: got=%d want=502", len(rows))
	}

	seen := make(map[string]struct{}, 501)
	for _, row := range rows[1:] {
		if _, exists := seen[row[0]]; exists {
			t.Fatalf("duplicate request log id exported: %s", row[0])
		}
		seen[row[0]] = struct{}{}
	}
	if _, exists := seen["9999"]; exists {
		t.Fatal("newly inserted top row should not enter current export snapshot")
	}
	if len(seen) != 501 {
		t.Fatalf("unexpected exported unique rows: got=%d want=501", len(seen))
	}
}

func TestHasRequestLogExportRowsUsesExportFilters(t *testing.T) {
	db := newRequestLogExportTestDB(t)
	restoreGetDB := requestLogGetDB
	requestLogGetDB = func(model ...any) (*gorm.DB, error) { return db, nil }
	defer func() { requestLogGetDB = restoreGetDB }()

	params := &form.RequestLogExport{RequestID: "missing", Limit: 1000}
	hasRows, err := HasRequestLogExportRows(context.Background(), params)
	if err != nil || hasRows {
		t.Fatalf("empty filter must not be exportable: hasRows=%t err=%v", hasRows, err)
	}

	if err := db.Exec(`INSERT INTO request_logs (id, request_id, created_at, updated_at) VALUES (?, ?, ?, ?)`, 1, "matching", time.Now(), time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	params.RequestID = "matching"
	hasRows, err = HasRequestLogExportRows(context.Background(), params)
	if err != nil || !hasRows {
		t.Fatalf("matching filter must be exportable: hasRows=%t err=%v", hasRows, err)
	}
	params.HasChangeDiff = 1
	hasRows, err = HasRequestLogExportRows(context.Background(), params)
	if err != nil || hasRows {
		t.Fatalf("change-diff filter must match export semantics: hasRows=%t err=%v", hasRows, err)
	}
}

func newRequestLogExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE request_logs (
		id integer primary key,
		request_id text,
		operator_account text,
		operator_name text,
		ip text,
		request_kind text,
		execution_time_scope text,
		method text,
		base_url text,
		operation_name text,
		operation_status integer,
		is_high_risk integer,
		response_status integer,
		execution_time_us integer,
		change_diff text,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatalf("create request_logs failed: %v", err)
	}
	return db
}
