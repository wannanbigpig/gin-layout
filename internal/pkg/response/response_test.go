package response

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
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
