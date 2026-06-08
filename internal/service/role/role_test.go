package role

import (
	"errors"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestGenerateRoleCodeUsesUniqueDefaultPrefix(t *testing.T) {
	service := NewRoleService()

	first := service.generateRoleCode()
	second := service.generateRoleCode()

	if first == "" || second == "" {
		t.Fatal("expected generated role code")
	}
	if first == second {
		t.Fatalf("expected different role codes, got %s", first)
	}
	if first[:5] != "role_" || second[:5] != "role_" {
		t.Fatalf("expected role_ prefix, got %s and %s", first, second)
	}
}

func TestIsRoleCodeDuplicateErrRecognizesMySQLDuplicate(t *testing.T) {
	err := &mysqlDriver.MySQLError{
		Number:  1062,
		Message: "Duplicate entry '12-0' for key 'role.uniq_code_deleted_at'",
	}
	if !isRoleCodeDuplicateErr(err) {
		t.Fatal("expected role duplicate error to be recognized")
	}
}

func TestIsRoleCodeDuplicateErrIgnoresOtherErrors(t *testing.T) {
	err := &mysqlDriver.MySQLError{
		Number:  1062,
		Message: "Duplicate entry '12-0' for key 'menu.uniq_code_deleted_at'",
	}
	if isRoleCodeDuplicateErr(err) {
		t.Fatal("expected non-role duplicate error to be ignored")
	}
}

func TestUpdateRejectsImmutableCodeBeforeDBLookup(t *testing.T) {
	code := "auditor_new"
	err := NewRoleService().Update(&form.UpdateRole{Id: 1, Code: &code})

	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected business error, got %T: %v", err, err)
	}
	if businessErr.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, businessErr.GetCode())
	}
}
