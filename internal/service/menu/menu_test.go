package menu

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
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
	expected := "(title like ? OR path like ? OR code = ?) AND is_auth = ? AND status = ?"
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

func TestApplyDescendantMenuState(t *testing.T) {
	service := NewMenuService()
	parent := &model.Menu{Pids: "0,1", Level: 3, FullPath: "/system"}
	parent.ID = 10
	child := &model.Menu{Path: "users", Type: model.MENU}

	service.applyDescendantMenuState(parent, child)
	if child.Pids != "0,1,10" {
		t.Fatalf("unexpected child pids: %s", child.Pids)
	}
	if child.Level != 4 {
		t.Fatalf("unexpected child level: %d", child.Level)
	}
	if child.FullPath != "/system/users" {
		t.Fatalf("unexpected child full path: %s", child.FullPath)
	}

	buttonChild := &model.Menu{Path: "submit", Type: model.BUTTON}
	service.applyDescendantMenuState(parent, buttonChild)
	if buttonChild.FullPath != "" {
		t.Fatalf("expected empty full path for button child, got %s", buttonChild.FullPath)
	}
}
