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
