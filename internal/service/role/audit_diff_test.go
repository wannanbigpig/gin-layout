package role

import (
	"encoding/json"
	"testing"
)

func TestBuildRoleDiffIncludesStatusDisplay(t *testing.T) {
	raw := buildRoleDiff(
		map[string]any{"status": uint8(0)},
		map[string]any{"status": uint8(1)},
	)
	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("expected valid json diff, got err=%v raw=%s", err, raw)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 diff item, got %d", len(items))
	}
	if items[0]["before_display"] != "禁用" || items[0]["after_display"] != "启用" {
		t.Fatalf("unexpected status display mapping: %#v", items[0])
	}
}
