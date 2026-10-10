package auditdiff

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
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

func TestBuildFieldDiffMasksSensitiveFields(t *testing.T) {
	items := BuildFieldDiff(
		map[string]any{
			"phone_number": "13800138000",
			"email":        "admin@example.com",
			"password":     "SecretPass123",
			"nickname":     "OldNick",
		},
		map[string]any{
			"phone_number": "13900139000",
			"email":        "newadmin@example.com",
			"password":     "NewSecretPass456",
			"nickname":     "NewNick",
		},
		[]FieldRule{
			{Field: "phone_number", Label: "手机号"},
			{Field: "email", Label: "邮箱"},
			{Field: "password", Label: "密码"},
			{Field: "nickname", Label: "昵称"},
		},
	)

	if len(items) != 4 {
		t.Fatalf("expected 4 diff items, got %d", len(items))
	}

	diffMap := make(map[string]ChangeDiffItem)
	for _, it := range items {
		diffMap[it.Field] = it
	}

	// nickname 普通字段不脱敏
	if diffMap["nickname"].Before != "OldNick" || diffMap["nickname"].After != "NewNick" {
		t.Fatalf("unexpected nickname diff: %+v", diffMap["nickname"])
	}

	// 手机号自动掩码为前3后4
	if diffMap["phone_number"].Before != "138****8000" || diffMap["phone_number"].After != "139****9000" {
		t.Fatalf("expected masked phone numbers, got: %+v", diffMap["phone_number"])
	}

	// 邮箱自动掩码
	if diffMap["email"].Before != "ad***@example.com" || diffMap["email"].After != "ne***@example.com" {
		t.Fatalf("expected masked email, got: %+v", diffMap["email"])
	}

	// 密码自动掩码且绝不包含原始明文
	if diffMap["password"].Before == "SecretPass123" || diffMap["password"].After == "NewSecretPass456" {
		t.Fatalf("expected password to be masked, got: %+v", diffMap["password"])
	}
	if !strings.Contains(diffMap["password"].Before.(string), "*") {
		t.Fatalf("expected masked password with asterisks, got: %+v", diffMap["password"])
	}
}

func TestBuildFieldDiffUsesCommonNameMaskConfiguration(t *testing.T) {
	previous := sensitive.GetSensitiveFieldsConfig()
	t.Cleanup(func() { sensitive.LoadSensitiveFieldsConfig(previous) })

	rules := []FieldRule{{Field: "name", Label: "角色名称"}}
	sensitive.LoadSensitiveFieldsConfig(sensitive.DefaultSensitiveFieldsConfig())
	plain := BuildFieldDiff(nil, map[string]any{"name": "但是"}, rules)
	if len(plain) != 1 || plain[0].After != "但是" {
		t.Fatalf("expected plain role name with default fields, got %#v", plain)
	}

	config := sensitive.DefaultSensitiveFieldsConfig()
	config.Common = append(config.Common, "name")
	sensitive.LoadSensitiveFieldsConfig(config)
	masked := BuildFieldDiff(nil, map[string]any{"name": "但是"}, rules)
	if len(masked) != 1 || masked[0].After != "**" {
		t.Fatalf("expected configured name masking, got %#v", masked)
	}
}
