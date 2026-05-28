package audit

import (
	"strings"
	"testing"
)

func TestResolveRequestLogOrderByDefaultsToCreatedAtDesc(t *testing.T) {
	orderBy := resolveRequestLogOrderBy("")
	if orderBy != "created_at DESC, id DESC" {
		t.Fatalf("unexpected default order by: %s", orderBy)
	}
}

func TestResolveRequestLogOrderByAppendsStableTieBreaker(t *testing.T) {
	orderBy := resolveRequestLogOrderBy("execution_time desc")
	if orderBy != "execution_time desc, id DESC" {
		t.Fatalf("unexpected order by: %s", orderBy)
	}
}

func TestRequestLogOrderAllowMapIncludesExecutionTime(t *testing.T) {
	if _, ok := requestLogOrderAllowMap["execution_time"]; !ok {
		t.Fatal("expected execution_time to be sortable")
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
