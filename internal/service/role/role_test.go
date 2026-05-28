package role

import (
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
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
