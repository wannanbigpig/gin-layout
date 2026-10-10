package admin_test

import (
	"net/http"
	"testing"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestProfileSecurityOptionsReturnsCurrentPolicy(t *testing.T) {
	resp := getRequest("/admin/v1/admin-user/profile-security/options", nil)
	result := decodeResult(t, resp)
	if resp.Code != http.StatusOK || result.Code != e.SUCCESS {
		t.Fatalf("options failed: HTTP=%d code=%d", resp.Code, result.Code)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected options shape: %T", result.Data)
	}
	if _, ok := data["sms_verification_required"].(bool); !ok {
		t.Fatalf("missing boolean policy: %#v", data)
	}
	anonymous := anonymousGetRequest("/admin/v1/admin-user/profile-security/options", nil)
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous options HTTP=%d", anonymous.Code)
	}
}

func TestProfileSecurityRejectsInvalidRequestsBeforeSendingOrChangingPhone(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"invalid OTP purpose", "/admin/v1/admin-user/profile-security/send-code", `{"purpose":"login","current_password":"123456"}`},
		{"phone without password", "/admin/v1/admin-user/change-phone", `{"country_code":"86","phone_number":"13800138000"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := postRequest(tc.path, &tc.body)
			result := decodeResult(t, resp)
			if resp.Code != http.StatusBadRequest || result.Code == e.SUCCESS {
				t.Fatalf("invalid request accepted: HTTP=%d code=%d", resp.Code, result.Code)
			}
			anonymous := anonymousPostRequest(tc.path, &tc.body)
			if anonymous.Code != http.StatusUnauthorized {
				t.Fatalf("anonymous request HTTP=%d", anonymous.Code)
			}
		})
	}
}
