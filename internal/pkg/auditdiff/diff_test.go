package auditdiff

import (
	"encoding/json"
	"testing"
)

func TestBuildFieldDiffMapsStatusLabel(t *testing.T) {
	items := BuildFieldDiff(
		map[string]any{"status": uint8(0), "remark": "old"},
		map[string]any{"status": uint8(1), "remark": "old"},
		[]FieldRule{
			{
				Field: "status",
				Label: "状态",
				ValueLabels: map[string]string{
					"0": "禁用",
					"1": "启用",
				},
			},
			{Field: "remark", Label: "备注"},
		},
	)
	if len(items) != 1 {
		t.Fatalf("expected 1 diff item, got %d", len(items))
	}
	if items[0].Field != "status" {
		t.Fatalf("expected field status, got %s", items[0].Field)
	}
	if items[0].BeforeDisplay != "禁用" || items[0].AfterDisplay != "启用" {
		t.Fatalf("unexpected display mapping: before=%s after=%s", items[0].BeforeDisplay, items[0].AfterDisplay)
	}
}

func TestMarshalReturnsJSONString(t *testing.T) {
	raw := Marshal([]ChangeDiffItem{{
		Field:  "status",
		Label:  "状态",
		Before: 0,
		After:  1,
	}})
	if raw == "" {
		t.Fatal("expected non-empty json")
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 decoded item, got %d", len(decoded))
	}
}

func TestMarshalReturnsEmptyArrayWhenNoDiff(t *testing.T) {
	if got := Marshal(nil); got != "[]" {
		t.Fatalf("expected [] for nil diff, got %s", got)
	}
}
