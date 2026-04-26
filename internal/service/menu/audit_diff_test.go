package menu

import (
	"encoding/json"
	"testing"
)

func TestBuildMenuDiffIncludesTypeDisplay(t *testing.T) {
	raw := buildMenuDiff(
		map[string]any{"type": uint8(1)},
		map[string]any{"type": uint8(2)},
	)
	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("expected valid json diff, got err=%v raw=%s", err, raw)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 diff item, got %d", len(items))
	}
	if items[0]["before_display"] != "目录" || items[0]["after_display"] != "菜单" {
		t.Fatalf("unexpected type display mapping: %#v", items[0])
	}
}
