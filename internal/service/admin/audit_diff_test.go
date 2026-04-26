package admin

import (
	"encoding/json"
	"testing"
)

func TestBuildAdminUserDiffIncludesStatusDisplay(t *testing.T) {
	raw := buildAdminUserDiff(
		map[string]any{
			"id":       uint(1),
			"status":   uint8(0),
			"role_ids": []uint{1},
		},
		map[string]any{
			"id":       uint(1),
			"status":   uint8(1),
			"role_ids": []uint{1, 2},
		},
	)
	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("expected valid json diff, got err=%v raw=%s", err, raw)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 diff items, got %d", len(items))
	}
	for _, item := range items {
		if item["field"] != "status" {
			continue
		}
		if item["before_display"] != "禁用" || item["after_display"] != "启用" {
			t.Fatalf("unexpected status display mapping: %#v", item)
		}
		return
	}
	t.Fatalf("expected status diff item, got %#v", items)
}
