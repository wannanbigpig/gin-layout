package task

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestBuildAsyncScanRowsMarksMissingDefinitions(t *testing.T) {
	taskTypes := []string{"audit:request_log.write", "demo:send"}
	builtin := map[string]model.TaskDefinition{
		"audit:request_log.write": {
			Code:  "audit:request_log.write",
			Queue: "audit",
		},
	}
	dbDefs := map[string]model.TaskDefinition{
		"audit:request_log.write": {
			Code:  "audit:request_log.write",
			Queue: "audit",
		},
	}

	rows := buildAsyncScanRows(taskTypes, builtin, dbDefs, true)
	if len(rows) != 2 {
		t.Fatalf("unexpected row count: %d", len(rows))
	}
	if rows[0].TaskType != "audit:request_log.write" || !rows[0].InBuiltin || !rows[0].InDB {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
	if rows[1].TaskType != "demo:send" || rows[1].InBuiltin || rows[1].InDB {
		t.Fatalf("unexpected second row: %+v", rows[1])
	}
}

func TestBuildAsyncScanRowsSkipsDBStateWhenDBNotReady(t *testing.T) {
	taskTypes := []string{"audit:request_log.write"}
	builtin := map[string]model.TaskDefinition{
		"audit:request_log.write": {
			Code:  "audit:request_log.write",
			Queue: "audit",
		},
	}
	dbDefs := map[string]model.TaskDefinition{
		"audit:request_log.write": {
			Code:  "audit:request_log.write",
			Queue: "audit",
		},
	}

	rows := buildAsyncScanRows(taskTypes, builtin, dbDefs, false)
	if len(rows) != 1 {
		t.Fatalf("unexpected row count: %d", len(rows))
	}
	if rows[0].InDB {
		t.Fatalf("expected InDB=false when dbReady=false, got %+v", rows[0])
	}
}
