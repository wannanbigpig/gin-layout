package form

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	validatorx "github.com/wannanbigpig/gin-layout/internal/validator"
)

func TestListMenuAllowsBinaryEnumFilters(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?is_auth=1&status=0", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewMenuListQuery()
	if err := ctx.ShouldBindQuery(payload); err != nil {
		t.Fatalf("expected is_auth/status in [0,1] to pass validation, got %v", err)
	}
}

func TestListMenuRejectsInvalidIsAuth(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?is_auth=2", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewMenuListQuery()
	if err := ctx.ShouldBindQuery(payload); err == nil {
		t.Fatal("expected is_auth=2 to fail validation")
	}
}

func TestListMenuRejectsInvalidStatus(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?status=2", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewMenuListQuery()
	if err := ctx.ShouldBindQuery(payload); err == nil {
		t.Fatal("expected status=2 to fail validation")
	}
}
