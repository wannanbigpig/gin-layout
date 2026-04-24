package validator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

type phonePayload struct {
	Phone string `json:"phone" form:"phone" label:"手机号" binding:"required,phone_number"`
}

type validationResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func TestValidationErrorUsesRequestLocale(t *testing.T) {
	resetValidatorRuntimeForI18nTest(t)
	if err := InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}

	zhMsg := validatePayloadAndReadMessage(t, "zh-CN")
	if zhMsg != "手机号格式不正确" {
		t.Fatalf("expected zh translation, got %q", zhMsg)
	}

	enMsg := validatePayloadAndReadMessage(t, "en-US")
	if enMsg != "手机号 format is invalid" {
		t.Fatalf("expected en translation, got %q", enMsg)
	}
}

func TestValidationErrorFallsBackToDefaultTranslator(t *testing.T) {
	resetValidatorRuntimeForI18nTest(t)
	if err := InitValidatorTrans("invalid-locale"); err != nil {
		t.Fatalf("init validator fallback: %v", err)
	}

	msg := validatePayloadAndReadMessage(t, "fr-FR")
	if msg != "手机号格式不正确" {
		t.Fatalf("expected fallback zh translation, got %q", msg)
	}
}

func validatePayloadAndReadMessage(t *testing.T, locale string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)

	body := strings.NewReader(`{"phone":"123"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", locale)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "validator-i18n")
	ctx.Set(global.ContextKeyLocale, locale)

	payload := &phonePayload{}
	if err := CheckPostParams(ctx, payload); err == nil {
		t.Fatal("expected validation error")
	}

	var result validationResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return result.Msg
}

func resetValidatorRuntimeForI18nTest(t *testing.T) {
	t.Helper()
	validatorRuntime = newValidatorRuntime()
	regexCache = sync.Map{}
}
