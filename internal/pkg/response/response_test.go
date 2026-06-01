package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestSuccessDefaultsDataToEmptyObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-1")

	Resp().Success(ctx)

	var result Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object data, got %#v", result.Data)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty object, got %#v", data)
	}
}

func TestWithNilDataReturnsEmptyObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-2")

	Resp().WithDataSuccess(ctx, nil)

	var result Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object data, got %#v", result.Data)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty object, got %#v", data)
	}
}

func TestScalarDataStillWrapped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-3")

	Resp().WithDataSuccess(ctx, true)

	var result struct {
		Data struct {
			Result bool `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !result.Data.Result {
		t.Fatalf("expected wrapped scalar result=true")
	}
}

func TestInt64DataStillWrapped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-4")

	Resp().WithDataSuccess(ctx, int64(42))

	var result struct {
		Data struct {
			Result int64 `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if result.Data.Result != 42 {
		t.Fatalf("expected wrapped scalar result=42, got %d", result.Data.Result)
	}
}

func TestTypedNilPointerReturnsEmptyObject(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-5")

	var nilPayload *payload
	Resp().WithDataSuccess(ctx, nilPayload)

	var result Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object data, got %#v", result.Data)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty object, got %#v", data)
	}
}

func TestNilSliceReturnsEmptyObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-6")

	var list []int
	Resp().WithDataSuccess(ctx, list)

	var result Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object data, got %#v", result.Data)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty object, got %#v", data)
	}
}

func TestSliceDataWrappedAsObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-7")

	Resp().WithDataSuccess(ctx, []int{1, 2, 3})

	var result struct {
		Data struct {
			Result []int `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(result.Data.Result) != 3 {
		t.Fatalf("expected wrapped slice length=3, got %d", len(result.Data.Result))
	}
}

func TestInferHTTPStatusContract(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		wantStatus int
	}{
		{"SUCCESS -> 200", errors.SUCCESS, 200},
		{"业务失败默认 -> 400", errors.FAILURE, 400},
		{"NotFound -> 404", errors.NotFound, 404},
		{"NotLogin -> 401", errors.NotLogin, 401},
		{"AuthorizationErr -> 403", errors.AuthorizationErr, 403},
		{"ServerErr -> 500", errors.ServerErr, 500},
		{"InvalidParameter -> 400", errors.InvalidParameter, 400},
		{"ServiceDependencyNotReady -> 503", errors.ServiceDependencyNotReady, 503},
		{"CasbinInitFailed -> 500", errors.CasbinInitFailed, 500},
		{"TokenGenerateFailed -> 500", errors.TokenGenerateFailed, 500},
		{"UserPasswordWrong -> 401", errors.UserPasswordWrong, 401},
		{"LoginAccountLocked -> 401", errors.LoginAccountLocked, 401},
		{"未登记业务码 -> 400", 99999, 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errors.InferHTTPStatus(tt.code)
			if got != tt.wantStatus {
				t.Errorf("InferHTTPStatus(%d) = %d, want %d", tt.code, got, tt.wantStatus)
			}
		})
	}
}

func TestSetHttpCodeOverridesInferredStatus(t *testing.T) {
	// 场景：NoRoute 使用 SetHttpCode(404) 覆盖 NotFound 推断（二者一致，验证链路生效）
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-override")

	Resp().SetHttpCode(http.StatusNotFound).FailCode(ctx, errors.NotFound)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 after SetHttpCode override, got %d", recorder.Code)
	}
}

func TestSetHttpCodeOverridesWhenValuesDiffer(t *testing.T) {
	// 场景：SetHttpCode(200) 覆盖 InferHTTPStatus 的推断值（未登记码默认 400），
	// 验证 httpCodeSet 标志生效，SetCode 不会覆盖显式设置的 HTTP 状态码。
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-diff")

	Resp().SetHttpCode(http.StatusOK).FailCode(ctx, 99999) // 99999 未登记，InferHTTPStatus→400

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 (SetHttpCode wins), got %d", recorder.Code)
	}
}

func TestFailCodeWithCustomMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-custom-msg")

	Resp().FailCode(ctx, errors.ServerErr, "自定义错误消息")

	var result Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if result.Msg != "自定义错误消息" {
		t.Fatalf("expected custom message, got %q", result.Msg)
	}
	if recorder.Code != 500 {
		t.Fatalf("expected HTTP 500 for ServerErr, got %d", recorder.Code)
	}
}

func TestFailCodeByKeyResolvesMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-key")
	ctx.Set(global.ContextKeyLocale, "zh-CN")

	Resp().FailCodeByKey(ctx, errors.ServerErr, errors.MsgKeyAuthPermissionInitFailed)

	var result Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if result.Code != errors.ServerErr {
		t.Fatalf("expected code %d, got %d", errors.ServerErr, result.Code)
	}
	if result.Msg != "权限验证初始化失败" {
		t.Fatalf("expected translated key message, got %q", result.Msg)
	}
}
