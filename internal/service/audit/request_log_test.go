package audit

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestRequestLogDetailPreservesDatabaseFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = (&RequestLogService{db: db}).Detail(1)
	if err == nil {
		t.Fatal("expected database error")
	}
	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) && businessErr.GetCode() == e.NotFound {
		t.Fatalf("database failure must not be reported as NotFound: %v", err)
	}
}

func TestResolveRequestLogOrderByDefaultsToCreatedAtDesc(t *testing.T) {
	orderBy := resolveRequestLogOrderBy("")
	if orderBy != "created_at DESC, id DESC" {
		t.Fatalf("unexpected default order by: %s", orderBy)
	}
}

func TestResolveRequestLogOrderByAppendsStableTieBreaker(t *testing.T) {
	orderBy := resolveRequestLogOrderBy("execution_time_us desc")
	if orderBy != "execution_time_us desc, id DESC" {
		t.Fatalf("unexpected order by: %s", orderBy)
	}
}

func TestRequestLogOrderAllowMapIncludesExecutionTime(t *testing.T) {
	if _, ok := requestLogOrderAllowMap["execution_time_us"]; !ok {
		t.Fatal("expected execution_time_us to be sortable")
	}
}

func TestBuildRequestLogQueryIncludesRequestKindFilter(t *testing.T) {
	query := buildRequestLogQuery(requestLogQueryInput{RequestKind: "websocket"})
	condition, args := query.Build()
	if !strings.Contains(condition, "request_kind = ?") {
		t.Fatalf("expected request_kind filter in condition, got %q", condition)
	}
	if len(args) != 1 || args[0] != "websocket" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestBuildRequestLogQueryIncludesExecutionTimeScopeFilter(t *testing.T) {
	query := buildRequestLogQuery(requestLogQueryInput{ExecutionTimeScope: "stream"})
	condition, args := query.Build()
	if !strings.Contains(condition, "execution_time_scope = ?") {
		t.Fatalf("expected execution_time_scope filter in condition, got %q", condition)
	}
	if len(args) != 1 || args[0] != "stream" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestRequestLogChangeFilterExcludesEmptyDiffs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE request_logs (id INTEGER PRIMARY KEY, change_diff text)").Error; err != nil {
		t.Fatal(err)
	}
	for _, diff := range []any{nil, "", "[]", " {} ", `[{"field":"name","before":"old","after":"new"}]`} {
		if err := db.Exec("INSERT INTO request_logs (change_diff) VALUES (?)", diff).Error; err != nil {
			t.Fatal(err)
		}
	}
	condition, args := buildRequestLogQuery(requestLogQueryInput{HasChangeDiff: 1}).Build()
	var ids []uint
	if err := db.Table("request_logs").Where(condition, args...).Pluck("id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 5 {
		t.Fatalf("unexpected change records: %v", ids)
	}
}

func TestRequestLogFailureFilterMatchesEveryNonZeroBusinessCode(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE request_logs (id INTEGER PRIMARY KEY, operation_status integer)").Error; err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{0, 1, 400, 10001} {
		if err := db.Exec("INSERT INTO request_logs (operation_status) VALUES (?)", status).Error; err != nil {
			t.Fatal(err)
		}
	}
	failure := 1
	condition, args := buildRequestLogQuery(requestLogQueryInput{OperationStatus: &failure}).Build()
	var statuses []int
	if err := db.Table("request_logs").Where(condition, args...).Order("id").Pluck("operation_status", &statuses).Error; err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 3 || statuses[0] != 1 || statuses[1] != 400 || statuses[2] != 10001 {
		t.Fatalf("failure filter statuses = %v, want every non-zero code", statuses)
	}
}
