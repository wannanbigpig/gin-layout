package api_permission

import (
	"encoding/json"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
)

func TestAPIPermissionDiffIncludesAuthModeDisplay(t *testing.T) {
	items := auditdiff.BuildFieldDiff(
		map[string]any{"is_auth": uint8(0)},
		map[string]any{"is_auth": uint8(2)},
		apiPermissionDiffRules,
	)
	raw := auditdiff.Marshal(items)
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("expected valid json diff, got err=%v raw=%s", err, raw)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 diff item, got %d", len(decoded))
	}
	if decoded[0]["before_display"] != "无需登录" || decoded[0]["after_display"] != "需要登录且鉴权" {
		t.Fatalf("unexpected auth display mapping: %#v", decoded[0])
	}
}
