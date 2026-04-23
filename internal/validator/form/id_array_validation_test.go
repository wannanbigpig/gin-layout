package form

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	validatorx "github.com/wannanbigpig/gin-layout/internal/validator"
)

func bindJSONBody(t *testing.T, body string, payload any) error {
	t.Helper()
	if err := validatorx.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator: %v", err)
	}
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	return ctx.ShouldBind(payload)
}

func TestDeptBindRoleRejectsZeroRoleID(t *testing.T) {
	payload := NewDeptBindRole()
	err := bindJSONBody(t, `{"dept_id":1,"role_ids":[0]}`, payload)
	if err == nil {
		t.Fatal("expected role_ids containing 0 to fail validation")
	}
}

func TestAdminUserBindRoleRejectsZeroRoleID(t *testing.T) {
	payload := NewBindRole()
	err := bindJSONBody(t, `{"user_id":1,"role_ids":[0]}`, payload)
	if err == nil {
		t.Fatal("expected role_ids containing 0 to fail validation")
	}
}

func TestCreateRoleRejectsZeroMenuID(t *testing.T) {
	payload := NewCreateRoleForm()
	err := bindJSONBody(t, `{"name":"测试角色","menu_list":[0]}`, payload)
	if err == nil {
		t.Fatal("expected menu_list containing 0 to fail validation")
	}
}

func TestCreateMenuRejectsZeroAPIID(t *testing.T) {
	payload := NewCreateMenuForm()
	err := bindJSONBody(t, `{"title_i18n":{"zh-CN":"测试菜单"},"sort":1,"type":1,"api_list":[0]}`, payload)
	if err == nil {
		t.Fatal("expected api_list containing 0 to fail validation")
	}
}

func TestIDArraysAllowPositiveValues(t *testing.T) {
	if err := bindJSONBody(t, `{"dept_id":1,"role_ids":[1]}`, NewDeptBindRole()); err != nil {
		t.Fatalf("expected positive dept role_ids to pass, got %v", err)
	}
	if err := bindJSONBody(t, `{"user_id":1,"role_ids":[1]}`, NewBindRole()); err != nil {
		t.Fatalf("expected positive user role_ids to pass, got %v", err)
	}
	if err := bindJSONBody(t, `{"name":"测试角色","menu_list":[1]}`, NewCreateRoleForm()); err != nil {
		t.Fatalf("expected positive menu_list to pass, got %v", err)
	}
	if err := bindJSONBody(t, `{"title_i18n":{"zh-CN":"测试菜单"},"sort":1,"type":1,"api_list":[1]}`, NewCreateMenuForm()); err != nil {
		t.Fatalf("expected positive api_list to pass, got %v", err)
	}
}
