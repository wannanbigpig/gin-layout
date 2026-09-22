package sensitive

import (
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeSensitiveFieldsConfigRemovesDuplicatesCoveredByCommon(t *testing.T) {
	config := NormalizeSensitiveFieldsConfig(SensitiveFieldsConfig{
		Common:         []string{" Password ", "token"},
		RequestHeader:  []string{"authorization", "Token", "authorization"},
		RequestBody:    []string{"password", "mobile", "TOKEN"},
		ResponseHeader: []string{"set-cookie", "token"},
		ResponseBody:   []string{"token", "idcard"},
	})

	if got, want := config.Common, []string{"password", "token"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected common fields: got=%v want=%v", got, want)
	}
	if got, want := config.RequestHeader, []string{"authorization"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected request_header fields: got=%v want=%v", got, want)
	}
	if got, want := config.RequestBody, []string{"mobile"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected request_body fields: got=%v want=%v", got, want)
	}
	if got, want := config.ResponseHeader, []string{"set-cookie"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected response_header fields: got=%v want=%v", got, want)
	}
	if got, want := config.ResponseBody, []string{"idcard"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected response_body fields: got=%v want=%v", got, want)
	}
}

func TestLoadSensitiveFieldsConfigAppliesCommonFieldsToRequestScopes(t *testing.T) {
	previous := GetSensitiveFieldsConfig()
	t.Cleanup(func() {
		LoadSensitiveFieldsConfig(previous)
	})

	LoadSensitiveFieldsConfig(SensitiveFieldsConfig{
		Common:        []string{"password", "token"},
		RequestHeader: []string{"authorization"},
		RequestBody:   []string{"mobile"},
	})

	maskedHeader := GetMaskedRequestHeaders(http.Header{
		"Token":         []string{"plain-token"},
		"Authorization": []string{"Bearer secret-token"},
	})
	if strings.Contains(maskedHeader, "plain-token") {
		t.Fatalf("expected common token field to be masked in request headers, got %s", maskedHeader)
	}
	if strings.Contains(maskedHeader, "secret-token") {
		t.Fatalf("expected authorization header to be masked, got %s", maskedHeader)
	}

	maskedBody := GetMaskedRequestBody([]byte(`{"password":"abc123","mobile":"13812345678","name":"张三"}`), "application/json")
	if strings.Contains(maskedBody, "abc123") {
		t.Fatalf("expected common password field to be masked in request body, got %s", maskedBody)
	}
	if strings.Contains(maskedBody, "13812345678") {
		t.Fatalf("expected request body specific mobile field to be masked, got %s", maskedBody)
	}

	current := GetSensitiveFieldsConfig()
	if got, want := current.RequestHeader, []string{"authorization"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected stored request_header config: got=%v want=%v", got, want)
	}
	if got, want := current.RequestBody, []string{"mobile"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected stored request_body config: got=%v want=%v", got, want)
	}
}

func TestImmutableSMSFieldsRemainMaskedAfterRuntimeConfigReplacement(t *testing.T) {
	previous := GetSensitiveFieldsConfig()
	t.Cleanup(func() { LoadSensitiveFieldsConfig(previous) })
	LoadSensitiveFieldsConfig(SensitiveFieldsConfig{RequestBody: []string{"email"}})

	masked := GetMaskedRequestBody([]byte(`{"providers":{"aliyun":{"access_key_secret":"aliyun-private-value"}},"phone_number":"13800138000","sms_code":"123456","captcha":"ABCD"}`), "application/json")
	for _, plaintext := range []string{"aliyun-private-value", "13800138000", "123456", "ABCD"} {
		if strings.Contains(masked, plaintext) {
			t.Fatalf("immutable SMS field leaked after runtime mask replacement: %s", masked)
		}
	}
}

// TestDefaultSensitiveFieldsRequestBodyExcludesGenericName 验证默认敏感字段列表收敛后，
// 通用 name 字段不再纳入默认脱敏；显式的真实姓名字段（real_name / realname）仍然脱敏。
func TestDefaultSensitiveFieldsRequestBodyExcludesGenericName(t *testing.T) {
	previous := GetSensitiveFieldsConfig()
	t.Cleanup(func() {
		LoadSensitiveFieldsConfig(previous)
	})
	LoadSensitiveFieldsConfig(DefaultSensitiveFieldsConfig())

	config := DefaultSensitiveFieldsConfig()

	for _, field := range config.RequestBody {
		if field == "name" {
			t.Fatalf("default request_body should not include generic name field: %#v", config.RequestBody)
		}
	}

	requireField := func(scope, target string, fields []string) {
		t.Helper()
		for _, field := range fields {
			if field == target {
				return
			}
		}
		t.Fatalf("default %s should still include %q, got=%v", scope, target, fields)
	}
	requireField("request_body", "real_name", config.RequestBody)
	requireField("request_body", "realname", config.RequestBody)

	for _, field := range config.ResponseBody {
		if field == "name" {
			t.Fatalf("default response_body should not include generic name field: %#v", config.ResponseBody)
		}
	}

	masked := GetMaskedRequestBody([]byte(`{"name":"张三","real_name":"李四"}`), "application/json")
	if !strings.Contains(masked, "张三") {
		t.Fatalf("expected generic name field to remain unmasked by default, got %s", masked)
	}
	if strings.Contains(masked, "李四") {
		t.Fatalf("expected real_name field to be masked by default, got %s", masked)
	}
}
