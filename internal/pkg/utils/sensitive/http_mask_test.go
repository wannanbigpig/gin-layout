package sensitive

import (
	"net/url"
	"strings"
	"testing"
)

func TestRequestBodyMasksFormCredentials(t *testing.T) {
	values := url.Values{"username": {"alice"}, "password": {"ExamplePwd!", "SecondPwd!"}, "token": {"opaque-token-value"}}
	out := GetMaskedRequestBody([]byte(values.Encode()), "application/x-www-form-urlencoded; charset=utf-8")
	masked, err := url.ParseQuery(out)
	if err != nil {
		t.Fatal(err)
	}
	if masked.Get("username") != "alice" {
		t.Fatal("non-sensitive field was lost")
	}
	for _, secret := range []string{"ExamplePwd!", "SecondPwd!", "opaque-token-value"} {
		if strings.Contains(out, secret) || strings.Contains(out, url.QueryEscape(secret)) {
			t.Fatal("form credential was logged in plaintext")
		}
	}
}

func TestRequestBodyMasksNestedStorageCredentials(t *testing.T) {
	previous := GetSensitiveFieldsConfig()
	defer LoadSensitiveFieldsConfig(previous)
	LoadSensitiveFieldsConfig(DefaultSensitiveFieldsConfig())
	raw := `{"config":{"aliyun_oss":{"access_key_secret":"storage-private-value","secret_access_key":"other-private-value","session_token":"temporary-private-value"}},"complete_token":"upload-private-value"}`
	masked := GetMaskedRequestBody([]byte(raw), "application/json")
	for _, secret := range []string{"storage-private-value", "other-private-value", "temporary-private-value", "upload-private-value"} {
		if strings.Contains(masked, secret) {
			t.Fatalf("credential leaked: %s", masked)
		}
	}
}

func TestRequestBodyDoesNotLogMalformedStructuredSecrets(t *testing.T) {
	for _, test := range []struct{ body, contentType string }{
		{`{"password":"ExamplePwd!","extra":1e1000}`, "application/json"},
		{`{"password":"ExamplePwd!",`, "application/json"},
		{"password=ExamplePwd!&bad=%zz", "application/x-www-form-urlencoded"},
	} {
		if got := GetMaskedRequestBody([]byte(test.body), test.contentType); strings.Contains(got, "ExamplePwd!") {
			t.Fatal("malformed structured body exposed credentials")
		}
	}
}

func TestMaskQueryString_JWTReadable(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.aHEQq0SignaturePart"
	raw := "client=admin-web&lang=zh-CN&token=" + jwt
	out := MaskQueryString(raw)

	// * 被 URL 编码为 %2A，不再还原
	if !strings.Contains(out, "token=eyJhbG%2A%2A%2A") {
		t.Fatalf("expected JWT to be masked with maskToken (6 prefix + %%2A%%2A%%2A + 6 suffix), got %q", out)
	}
	if strings.Contains(out, "eyJhbGciOiJI") {
		t.Fatalf("expected JWT alg header to be masked, got %q", out)
	}
}

func TestMaskDefault_FoldsLongStrings(t *testing.T) {
	short := "abcdefghij"
	if got := maskDefault(short); got != "a********j" {
		t.Fatalf("short string mask: want a********j, got %q", got)
	}

	long := strings.Repeat("x", 64)
	got := maskDefault(long)
	if got != "x***x" {
		t.Fatalf("long string should fold middle stars, want x***x, got %q", got)
	}
}

func TestMaskDefaultPreservesUTF8Characters(t *testing.T) {
	for input, want := range map[string]string{
		"但是":   "**",
		"测试角色": "测**色",
	} {
		if got := maskDefault(input); got != want {
			t.Errorf("maskDefault(%q) = %q, want %q", input, got, want)
		}
	}
}
