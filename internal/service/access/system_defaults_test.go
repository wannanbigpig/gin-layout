package access

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestSystemDefaultsServiceProtectedPredicates(t *testing.T) {
	service := NewSystemDefaultsService()

	protectedRole := model.NewRole()
	protectedRole.Code = global.SuperAdminRoleCode
	protectedRole.IsSystem = global.Yes
	if !service.IsProtectedRole(protectedRole) {
		t.Fatal("expected super admin role to be protected")
	}

	normalRole := model.NewRole()
	normalRole.Code = "editor"
	normalRole.IsSystem = global.No
	if service.IsProtectedRole(normalRole) {
		t.Fatal("expected normal role to be mutable")
	}

	protectedDept := model.NewDepartment()
	protectedDept.Code = global.DefaultDepartmentCode
	protectedDept.IsSystem = global.Yes
	if !service.IsProtectedDepartment(protectedDept) {
		t.Fatal("expected default department to be protected")
	}

	normalDept := model.NewDepartment()
	normalDept.Code = "sales"
	normalDept.IsSystem = global.No
	if service.IsProtectedDepartment(normalDept) {
		t.Fatal("expected normal department to be mutable")
	}
}

func TestRequireSuperAdminRoleForNonSuperAdminUserSkipsLookup(t *testing.T) {
	service := NewSystemDefaultsService()
	if err := service.RequireSuperAdminRoleForUser(2, nil); err != nil {
		t.Fatalf("expected non-super-admin user to skip protected role validation, got %v", err)
	}
}
