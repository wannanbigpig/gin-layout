package taskcenter

import (
	"encoding/json"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestBuildCancelAuditDiffContainsStatusMapping(t *testing.T) {
	raw := BuildCancelAuditDiff(&TaskRunAuditSnapshot{
		RunID:     101,
		TaskCode:  "demo:send",
		Status:    model.TaskRunStatusRunning,
		Queue:     "critical",
		SourceID:  "task-abc",
		HasRecord: true,
	}, map[string]any{
		"run_id":  uint(101),
		"task_id": "task-abc",
		"status":  model.TaskRunStatusCanceled,
	})

	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("expected valid json diff, got err=%v raw=%s", err, raw)
	}

	foundStatus := false
	for _, item := range items {
		if item["field"] != "status" {
			continue
		}
		foundStatus = true
		if item["before_display"] != "执行中" || item["after_display"] != "已取消" {
			t.Fatalf("unexpected status display mapping: %#v", item)
		}
	}
	if !foundStatus {
		t.Fatalf("expected status diff item, got %#v", items)
	}
}

func TestBuildTriggerAuditDiffContainsPayloadKeys(t *testing.T) {
	raw := BuildTriggerAuditDiff(&form.TaskTriggerForm{
		TaskCode: "cron:demo",
		Payload: map[string]any{
			"z": 1,
			"a": "x",
		},
	}, map[string]any{
		"run_id":  uint(1),
		"task_id": "manual:abc",
		"queue":   "default",
	})

	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("expected valid json diff, got err=%v raw=%s", err, raw)
	}

	foundPayloadKeys := false
	for _, item := range items {
		if item["field"] != "payload_keys" {
			continue
		}
		foundPayloadKeys = true
		after, ok := item["after"].([]any)
		if !ok || len(after) != 2 {
			t.Fatalf("unexpected payload_keys after value: %#v", item["after"])
		}
		if after[0] != "a" || after[1] != "z" {
			t.Fatalf("expected sorted payload keys [a z], got %#v", after)
		}
	}
	if !foundPayloadKeys {
		t.Fatalf("expected payload_keys diff item, got %#v", items)
	}
}
