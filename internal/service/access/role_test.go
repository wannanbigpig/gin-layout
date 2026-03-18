package access

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestRoleBuildListCondition(t *testing.T) {
	status := int8(1)
	pid := uint(8)
	params := &form.RoleList{
		Name:   "manager",
		Status: &status,
		Pid:    &pid,
	}

	condition, args := NewRoleService().buildListCondition(params)
	expected := "name like ? AND status = ? AND pid = ?"
	if condition != expected {
		t.Fatalf("unexpected condition: %s", condition)
	}
	if len(args) != 3 {
		t.Fatalf("unexpected args len: %d", len(args))
	}
}

func TestRoleBuildPids(t *testing.T) {
	service := NewRoleService()
	if got := service.buildPids("0", 2); got != "2" {
		t.Fatalf("unexpected root pids: %s", got)
	}
	if got := service.buildPids("1,2", 3); got != "1,2,3" {
		t.Fatalf("unexpected nested pids: %s", got)
	}
}

func TestRoleBuildPidsUpdateExpr(t *testing.T) {
	service := NewRoleService()
	rootExpr := service.buildPidsUpdateExpr("0", "5")
	if rootExpr == "" {
		t.Fatal("expected root expr")
	}

	nestedExpr := service.buildPidsUpdateExpr("1,2", "8,9")
	if nestedExpr == "" || nestedExpr == rootExpr {
		t.Fatalf("unexpected nested expr: %s", nestedExpr)
	}
}
