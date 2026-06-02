package form

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	validatorx "github.com/wannanbigpig/gin-layout/internal/validator"
)

func TestRequestLogListAllowsKnownHTTPMethod(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?method=GET", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewRequestLogListQuery()
	if err := ctx.ShouldBindQuery(payload); err != nil {
		t.Fatalf("expected method=GET to pass validation, got %v", err)
	}
}

func TestRequestLogListRejectsUnknownHTTPMethod(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?method=TRACE", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewRequestLogListQuery()
	if err := ctx.ShouldBindQuery(payload); err == nil {
		t.Fatal("expected method=TRACE to fail validation")
	}
}

func TestRequestLogListAllowsKnownRequestKind(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?request_kind=websocket", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewRequestLogListQuery()
	if err := ctx.ShouldBindQuery(payload); err != nil {
		t.Fatalf("expected request_kind=websocket to pass validation, got %v", err)
	}
}

func TestRequestLogListRejectsUnknownRequestKind(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?request_kind=grpc", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewRequestLogListQuery()
	if err := ctx.ShouldBindQuery(payload); err == nil {
		t.Fatal("expected request_kind=grpc to fail validation")
	}
}

func TestRequestLogListAllowsWebsocketAndConnectionScope(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?request_kind=websocket&execution_time_scope=connection", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewRequestLogListQuery()
	if err := ctx.ShouldBindQuery(payload); err != nil {
		t.Fatalf("expected websocket/connection query to pass validation, got %v", err)
	}
}

func TestRequestLogListRejectsRemovedSSEKind(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?request_kind=sse&execution_time_scope=stream", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewRequestLogListQuery()
	if err := ctx.ShouldBindQuery(payload); err == nil {
		t.Fatal("expected removed sse/stream values to fail validation")
	}
}

func TestRequestLogListRejectsUnknownExecutionTimeScope(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?execution_time_scope=duration", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewRequestLogListQuery()
	if err := ctx.ShouldBindQuery(payload); err == nil {
		t.Fatal("expected execution_time_scope=duration to fail validation")
	}
}
