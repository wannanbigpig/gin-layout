package sms

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAliyunProviderSendsSignedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("SMS parameters must not be placed in URL query: %s", r.URL.RawQuery)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("unexpected content type %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"Action", "PhoneNumbers", "SignName", "TemplateCode", "TemplateParam", "Signature"} {
			if r.Form.Get(key) == "" {
				t.Errorf("missing form field %s", key)
			}
		}
		if got := r.Form.Get("PhoneNumbers"); got != "13800138000" {
			t.Errorf("unexpected phone %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Code":"OK","RequestId":"request-1","BizId":"message-1"}`))
	}))
	defer server.Close()

	provider := &AliyunProvider{
		config: AliyunConfig{Enabled: true, AccessKeyID: "id", AccessKeySecret: "secret", Endpoint: server.URL},
		client: server.Client(),
	}
	provider.now = func() time.Time { return time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC) }
	provider.nonce = func() string { return "nonce" }
	result, err := provider.Send(context.Background(), SendInput{
		PhoneNumber: "+8613800138000", SignName: "GinLayout", TemplateCode: "SMS_LOGIN", Variables: map[string]string{"code": "123456"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != SendStateAccepted || result.ProviderRequestID != "request-1" || result.ProviderMessageID != "message-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

type leakingRoundTripper struct{}

func (leakingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	return nil, errors.New("transport failed url=" + req.URL.String() + " body=" + string(body))
}

func TestAliyunProviderTransportErrorDoesNotExposeRequestOrSensitiveParams(t *testing.T) {
	provider := &AliyunProvider{
		config: AliyunConfig{
			Enabled: true, AccessKeyID: "sensitive-ak", AccessKeySecret: "sensitive-secret", Endpoint: "https://sms.example.invalid",
		},
		client: &http.Client{Transport: leakingRoundTripper{}},
		now:    func() time.Time { return time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC) },
		nonce:  func() string { return "sensitive-nonce" },
	}
	result, err := provider.Send(context.Background(), SendInput{
		PhoneNumber: "+8613800138000", SignName: "GinLayout", TemplateCode: "SMS_LOGIN", Variables: map[string]string{"code": "123456"},
	})
	if err == nil || result.State != SendStateUnknown {
		t.Fatalf("expected sanitized unknown error, result=%#v err=%v", result, err)
	}
	for _, sensitive := range []string{
		"sms.example.invalid", "13800138000", "123456", "sensitive-ak", "sensitive-secret", "sensitive-nonce", "Signature=",
	} {
		if strings.Contains(err.Error(), sensitive) {
			t.Fatalf("transport error leaked %q: %v", sensitive, err)
		}
	}
}

func TestAliyunProviderRejectedResponseDoesNotExposeProviderPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"Code":"MOBILE_13800138000_CODE_123456","Message":"raw provider response"}`))
	}))
	defer server.Close()
	provider := &AliyunProvider{
		config: AliyunConfig{Enabled: true, AccessKeyID: "id", AccessKeySecret: "secret", Endpoint: server.URL},
		client: server.Client(), now: time.Now, nonce: func() string { return "nonce" },
	}
	result, err := provider.Send(context.Background(), SendInput{
		PhoneNumber: "+8613800138000", SignName: "GinLayout", TemplateCode: "SMS_LOGIN", Variables: map[string]string{"code": "123456"},
	})
	if err == nil || result.State != SendStateFailed {
		t.Fatalf("expected rejected result, result=%#v err=%v", result, err)
	}
	if strings.Contains(err.Error(), "13800138000") || strings.Contains(err.Error(), "123456") || strings.Contains(err.Error(), "raw provider response") {
		t.Fatalf("provider error leaked response payload: %v", err)
	}
}
