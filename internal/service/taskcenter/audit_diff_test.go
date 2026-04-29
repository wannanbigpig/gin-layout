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
		"run_id":              uint(101),
		"task_id":             "task-abc",
		"status":              model.TaskRunStatusCanceled,
		"canceled_by":         uint(1),
		"canceled_by_account": "tester",
		"cancel_reason":       "manual cancel",
	})

	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("expected valid json diff, got err=%v raw=%s", err, raw)
	}

	foundStatus := false
	foundAccount := false
	foundReason := false
	for _, item := range items {
		switch item["field"] {
		case "status":
			foundStatus = true
			if item["before_display"] != "执行中" || item["after_display"] != "已取消" {
				t.Fatalf("unexpected status display mapping: %#v", item)
			}
		case "canceled_by_account":
			foundAccount = true
			if item["after"] != "tester" {
				t.Fatalf("unexpected canceled_by_account item: %#v", item)
			}
		case "cancel_reason":
			foundReason = true
			if item["after"] != "manual cancel" {
				t.Fatalf("unexpected cancel_reason item: %#v", item)
			}
		}
	}
	if !foundStatus {
		t.Fatalf("expected status diff item, got %#v", items)
	}
	if !foundAccount || !foundReason {
		t.Fatalf("expected cancel audit meta items, got %#v", items)
	}
}

func TestBuildTriggerAuditDiffContainsPayloadKeys(t *testing.T) {
	raw := BuildTriggerAuditDiff(&form.TaskTriggerForm{
		TaskCode: "cron:demo",
		Confirm:  "CONFIRM",
		Reason:   "manual run",
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
	foundConfirm := false
	foundReason := false
	for _, item := range items {
		switch item["field"] {
		case "payload_keys":
			foundPayloadKeys = true
			after, ok := item["after"].([]any)
			if !ok || len(after) != 2 {
				t.Fatalf("unexpected payload_keys after value: %#v", item["after"])
			}
			if after[0] != "a" || after[1] != "z" {
				t.Fatalf("expected sorted payload keys [a z], got %#v", after)
			}
		case "confirm":
			foundConfirm = true
			if item["after"] != "CONFIRM" {
				t.Fatalf("unexpected confirm item: %#v", item)
			}
		case "reason":
			foundReason = true
			if item["after"] != "manual run" {
				t.Fatalf("unexpected reason item: %#v", item)
			}
		}
	}
	if !foundPayloadKeys {
		t.Fatalf("expected payload_keys diff item, got %#v", items)
	}
	if !foundConfirm || !foundReason {
		t.Fatalf("expected trigger audit meta items, got %#v", items)
	}
}
