package audit

import (
	"encoding/json"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
)

func TestDecodeMaskConfigNormalizesFields(t *testing.T) {
	config, err := decodeMaskConfig(`{
		"common": [" Password ", "password", "Token"],
		"request_header": ["authorization", "Authorization", ""],
		"request_body": ["mobile", "MOBILE"],
		"response_header": [],
		"response_body": ["IDCard", "idcard"]
	}`)
	if err != nil {
		t.Fatalf("decodeMaskConfig returned error: %v", err)
	}

	if got, want := len(config.Common), 2; got != want {
		t.Fatalf("unexpected common length: got=%d want=%d", got, want)
	}
	if config.Common[0] != "password" || config.Common[1] != "token" {
		t.Fatalf("unexpected normalized common fields: %#v", config.Common)
	}
	if got, want := len(config.RequestHeader), 1; got != want {
		t.Fatalf("unexpected request_header length: got=%d want=%d", got, want)
	}
	if config.RequestHeader[0] != "authorization" {
		t.Fatalf("unexpected request_header fields: %#v", config.RequestHeader)
	}
	if got, want := len(config.RequestBody), 1; got != want {
		t.Fatalf("unexpected request_body length: got=%d want=%d", got, want)
	}
	if config.RequestBody[0] != "mobile" {
		t.Fatalf("unexpected request_body fields: %#v", config.RequestBody)
	}
	if got, want := len(config.ResponseBody), 1; got != want {
		t.Fatalf("unexpected response_body length: got=%d want=%d", got, want)
	}
	if config.ResponseBody[0] != "idcard" {
		t.Fatalf("unexpected response_body fields: %#v", config.ResponseBody)
	}
}

func TestDecodeMaskConfigEmptyUsesDefault(t *testing.T) {
	config, err := decodeMaskConfig("   ")
	if err != nil {
		t.Fatalf("decodeMaskConfig returned error: %v", err)
	}

	defaultConfig := sensitive.DefaultSensitiveFieldsConfig()
	if len(config.Common) != len(defaultConfig.Common) {
		t.Fatalf("unexpected default common length: got=%d want=%d", len(config.Common), len(defaultConfig.Common))
	}
	if len(config.RequestHeader) != len(defaultConfig.RequestHeader) {
		t.Fatalf("unexpected default request_header length: got=%d want=%d", len(config.RequestHeader), len(defaultConfig.RequestHeader))
	}
}

func TestBuildMaskConfigAuditDiff(t *testing.T) {
	before := map[string]any{
		"common":          []string{"password"},
		"request_header":  []string{"authorization"},
		"request_body":    []string{"mobile"},
		"response_header": []string{},
		"response_body":   []string{"idcard"},
	}
	after := map[string]any{
		"common":          []string{"password", "token"},
		"request_header":  []string{"authorization"},
		"request_body":    []string{"mobile"},
		"response_header": []string{},
		"response_body":   []string{"idcard"},
	}

	raw := buildMaskConfigAuditDiff(before, after)
	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("expected valid json diff, got err=%v raw=%s", err, raw)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 diff item, got %d", len(items))
	}
	if items[0]["field"] != "common" {
		t.Fatalf("expected diff field common, got %#v", items[0]["field"])
	}
}
