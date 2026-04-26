package dept

import (
	"encoding/json"
	"testing"
)

func TestBuildDeptDiffContainsRoleIDs(t *testing.T) {
	raw := buildDeptDiff(
		map[string]any{"id": uint(1), "role_ids": []uint{1}},
		map[string]any{"id": uint(1), "role_ids": []uint{1, 2}},
	)
	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("expected valid json diff, got err=%v raw=%s", err, raw)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 diff item, got %d", len(items))
	}
	if items[0]["field"] != "role_ids" {
		t.Fatalf("expected role_ids diff, got %#v", items[0]["field"])
	}
}
