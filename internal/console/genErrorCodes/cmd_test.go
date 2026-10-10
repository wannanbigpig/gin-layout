package genErrorCodes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	appErrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func writeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readDoc(t *testing.T, path string) outputDoc {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc outputDoc
	if err := json.Unmarshal(bytes, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestGenerateUsesSelectedSourceDescriptionsAndCommonFirst(t *testing.T) {
	src := t.TempDir()
	writeFixture(t, src, "code_auth.go", "package errors\nconst UserPasswordWrong = 20001\n")
	writeFixture(t, src, "code_common.go", "package errors\nconst ( SUCCESS = 0; FAILURE = 1; InvalidParameter = 10000 )\n")
	writeFixture(t, src, "zh-cn_auth.go", `package errors
func init() {
    mergeZhCNText(map[int]string{UserPasswordWrong: "所选版本的密码说明"})
    mergeZhCNTextKey(map[string]string{MsgKeyAuthSessionExpired: "会话文案不属于错误码"})
}`)
	writeFixture(t, src, "zh-cn_common.go", `package errors
func init() { mergeZhCNText(map[int]string{SUCCESS: "OK", FAILURE: "FAIL", InvalidParameter: "参数错误"}) }`)
	// 测试文件不参与正式版本文档。
	writeFixture(t, src, "zh-cn_auth_test.go", `package errors
func testMessages() { mergeZhCNText(map[int]string{UserPasswordWrong: "测试文案"}) }`)
	jsonPath, tsPath := filepath.Join(src, "codes.json"), filepath.Join(src, "codes.ts")
	if err := generate(src, jsonPath, tsPath); err != nil {
		t.Fatal(err)
	}
	doc := readDoc(t, jsonPath)
	if doc.Total != 4 || filepath.Base(doc.Groups[0].File) != "code_common.go" {
		t.Fatalf("unexpected code count or group ordering: %+v", doc)
	}
	if doc.Groups[0].Codes[0].Description != "成功" || doc.Groups[0].Codes[1].Description != "失败" || doc.Groups[1].Codes[0].Description != "所选版本的密码说明" {
		t.Fatalf("descriptions did not come from the selected source: %+v", doc.Groups)
	}
	bytes, err := os.ReadFile(tsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"/** 所选版本的密码说明 */", "UserPasswordWrong: 20001,", "export type ErrorCodeName = keyof typeof ErrorCode"} {
		if !strings.Contains(string(bytes), expected) {
			t.Fatalf("generated TypeScript missing %q", expected)
		}
	}
}

func TestGenerateRejectsIncompleteChineseDescriptionsBeforeWriting(t *testing.T) {
	for _, tt := range []struct {
		name, text, errorText string
	}{
		{"missing", "", "missing Chinese description"},
		{"empty", `func init() { mergeZhCNText(map[int]string{UserPasswordWrong: " "}) }`, "missing Chinese description"},
		{"duplicate", `func init() { mergeZhCNText(map[int]string{UserPasswordWrong: "密码错误"}); mergeZhCNText(map[int]string{UserPasswordWrong: "重复文案"}) }`, "duplicate Chinese error text"},
		{"unsupported", `func init() { mergeZhCNText(messages) }`, "map literal"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := t.TempDir()
			writeFixture(t, src, "code_auth.go", "package errors\nconst UserPasswordWrong = 20001\n")
			writeFixture(t, src, "zh-cn_auth.go", "package errors\n"+tt.text)
			jsonPath, tsPath := filepath.Join(src, "codes.json"), filepath.Join(src, "codes.ts")
			writeFixture(t, src, "codes.json", "existing JSON")
			writeFixture(t, src, "codes.ts", "existing TypeScript")
			if err := generate(src, jsonPath, tsPath); err == nil || !strings.Contains(err.Error(), tt.errorText) {
				t.Fatalf("expected %q, got %v", tt.errorText, err)
			}
			for file, expected := range map[string]string{jsonPath: "existing JSON", tsPath: "existing TypeScript"} {
				bytes, err := os.ReadFile(file)
				if err != nil || string(bytes) != expected {
					t.Fatalf("invalid input overwrote %s: %s, %v", file, bytes, err)
				}
			}
		})
	}
}

func TestRepositoryDescriptionsMatchRuntimeChineseMessages(t *testing.T) {
	src := filepath.Join("..", "..", "pkg", "errors")
	out := t.TempDir()
	jsonPath := filepath.Join(out, "codes.json")
	if err := generate(src, jsonPath, filepath.Join(out, "codes.ts")); err != nil {
		t.Fatal(err)
	}
	doc := readDoc(t, jsonPath)
	if len(doc.Groups) == 0 || filepath.Base(doc.Groups[0].File) != "code_common.go" {
		t.Fatal("common errors must be the first group")
	}
	text := appErrors.NewErrorText("zh_CN")
	for _, group := range doc.Groups {
		for _, code := range group.Codes {
			if !strings.ContainsFunc(code.Description, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Errorf("%s has no Chinese description: %q", code.Name, code.Description)
			}
			message := text.Text(code.Code)
			if code.Name == "SUCCESS" {
				if message != "OK" || code.Description != "成功" {
					t.Errorf("success protocol changed: %s / %s", message, code.Description)
				}
			} else if code.Name == "FAILURE" {
				if message != "FAIL" || code.Description != "失败" {
					t.Errorf("failure protocol changed: %s / %s", message, code.Description)
				}
			} else if code.Description != message {
				t.Errorf("%s description differs from runtime: %q / %q", code.Name, code.Description, message)
			}
		}
	}
}
