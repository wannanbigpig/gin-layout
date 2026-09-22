package api_permission

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestApiBuildListCondition(t *testing.T) {
	isAuth := int8(1)
	isEffective := int8(0)
	params := &form.ListPermission{
		Name:        "ping",
		Method:      "GET",
		Route:       "/ping",
		Keyword:     "svc",
		IsAuth:      &isAuth,
		IsEffective: &isEffective,
	}

	condition, args := NewApiService().buildListCondition(params)
	expected := "(name like ? OR name_i18n like ? OR route like ? OR code = ?) AND (name like ? OR name_i18n like ?) AND method = ? AND route like ? AND is_auth = ? AND is_effective = ?"
	if condition != expected {
		t.Fatalf("unexpected condition: %s", condition)
	}
	if len(args) != 10 {
		t.Fatalf("unexpected args len: %d", len(args))
	}
}
