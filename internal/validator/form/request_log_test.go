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

func TestRequestLogListScopeQueryValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scope := range []string{"", "regular", "all", "important", "changes", "failed", "high_risk", "self_reads"} {
		t.Run("valid_"+scope, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("GET", "/?scope="+scope, nil)
			params := NewRequestLogListQuery()
			if err := ctx.ShouldBindQuery(params); err != nil {
				t.Fatalf("scope %q should be valid: %v", scope, err)
			}
			if params.Scope != scope {
				t.Fatalf("scope = %q, want %q", params.Scope, scope)
			}
		})
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/?scope=unknown", nil)
	if err := ctx.ShouldBindQuery(NewRequestLogListQuery()); err == nil {
		t.Fatal("unknown scope should be rejected")
	}
}
