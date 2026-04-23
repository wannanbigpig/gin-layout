package menu

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestMenuBuildListCondition(t *testing.T) {
	isAuth := int8(1)
	status := int8(1)
	params := &form.ListMenu{
		Keyword: "dashboard",
		IsAuth:  &isAuth,
		Status:  &status,
	}

	condition, args := NewMenuService().buildListCondition(params, true)
	expected := "(id IN (SELECT menu_id FROM menu_i18n WHERE title like ?) OR path like ? OR code = ?) AND is_auth = ? AND status = ?"
	if condition != expected {
		t.Fatalf("unexpected condition: %s", condition)
	}
	if len(args) != 5 {
		t.Fatalf("unexpected args len: %d", len(args))
	}
}

func TestAssembleFullPath(t *testing.T) {
	service := NewMenuService()
	if got := service.buildFullPath("users", "/admin", model.MENU); got != "/admin/users" {
		t.Fatalf("unexpected full path: %s", got)
	}
	if got := service.buildFullPath("https://example.com", "/admin", model.MENU); got != "https://example.com" {
		t.Fatalf("unexpected external path: %s", got)
	}
	if got := service.buildFullPath("button", "/admin", model.BUTTON); got != "" {
		t.Fatalf("expected empty path for button, got %s", got)
	}
}

func TestBuildPids(t *testing.T) {
	service := NewMenuService()
	if got := service.buildPids("0,1", 10); got != "0,1,10" {
		t.Fatalf("unexpected pids: %s", got)
	}
	if got := service.buildPids("", 10); got != "10" {
		t.Fatalf("unexpected root pids: %s", got)
	}
}

func TestMenuLocalePriority(t *testing.T) {
	priorities := menuLocalePriority("ja-JP")
	if len(priorities) != 3 {
		t.Fatalf("unexpected priorities length: %d", len(priorities))
	}
	if priorities[0] != i18n.LocaleZhCN || priorities[2] != i18n.LocaleEnUS {
		t.Fatalf("unexpected priorities: %+v", priorities)
	}
}
