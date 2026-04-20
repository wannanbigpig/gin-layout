package form

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	validatorx "github.com/wannanbigpig/gin-layout/internal/validator"
)

func TestUpdatePermissionAllowsThreeStateIsAuth(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	body := `{"id":1,"name":"route-authz","description":"test","is_auth":2,"sort":100}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewUpdateApiForm()
	if err := ctx.ShouldBind(payload); err != nil {
		t.Fatalf("expected is_auth=2 to pass validation, got %v", err)
	}
}

func TestListPermissionAllowsThreeStateIsAuth(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/?is_auth=2", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewListApiQuery()
	if err := ctx.ShouldBindQuery(payload); err != nil {
		t.Fatalf("expected list is_auth=2 to pass validation, got %v", err)
	}
}

func TestUpdatePermissionRejectsUnknownIsAuth(t *testing.T) {
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	body := `{"id":1,"name":"route-invalid","is_auth":3,"sort":100}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	payload := NewUpdateApiForm()
	if err := ctx.ShouldBind(payload); err == nil {
		t.Fatal("expected is_auth=3 to fail validation")
	}
}
