package role

import (
	"errors"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
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

func TestUpdateRoleDataScopeDeptKeepsOnlyExistingActiveDepartments(t *testing.T) {
	db := newRoleSQLiteDB(t)
	activeDept := model.Department{Name: "active"}
	deletedDept := model.Department{
		Name: "deleted",
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			DeletedAt: soft_delete.DeletedAt(1),
		},
	}
	if err := db.Create(&activeDept).Error; err != nil {
		t.Fatalf("create active department failed: %v", err)
	}
	if err := db.Create(&deletedDept).Error; err != nil {
		t.Fatalf("create deleted department failed: %v", err)
	}
	if err := db.Create(&[]model.RoleDataScopeDept{
		{RoleId: 7, DeptId: deletedDept.ID},
		{RoleId: 7, DeptId: 999},
	}).Error; err != nil {
		t.Fatalf("create existing mappings failed: %v", err)
	}

	err := NewRoleService().updateRoleDataScopeDept(7, access.DataScopeCustom, []uint{activeDept.ID, deletedDept.ID, 1000}, db)
	if err != nil {
		t.Fatalf("update role data scope dept failed: %v", err)
	}

	var got []uint
	if err := db.Model(&model.RoleDataScopeDept{}).Where("role_id = ?", 7).Order("dept_id").Pluck("dept_id", &got).Error; err != nil {
		t.Fatalf("query mappings failed: %v", err)
	}
	if len(got) != 1 || got[0] != activeDept.ID {
		t.Fatalf("expected only active department mapping, got %v", got)
	}
}

func TestUpdateRoleDataScopeDeptClearsWhenScopeIsNotCustom(t *testing.T) {
	db := newRoleSQLiteDB(t)
	if err := db.Create(&model.RoleDataScopeDept{RoleId: 8, DeptId: 1}).Error; err != nil {
		t.Fatalf("create existing mapping failed: %v", err)
	}

	err := NewRoleService().updateRoleDataScopeDept(8, access.DataScopeDept, []uint{1}, db)
	if err != nil {
		t.Fatalf("update role data scope dept failed: %v", err)
	}

	var count int64
	if err := db.Model(&model.RoleDataScopeDept{}).Where("role_id = ?", 8).Count(&count).Error; err != nil {
		t.Fatalf("count mappings failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected mappings cleared, got count=%d", count)
	}
}

func newRoleSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := createRoleSQLiteTables(db); err != nil {
		t.Fatalf("migrate role tables failed: %v", err)
	}
	return db
}

func createRoleSQLiteTables(db *gorm.DB) error {
	if err := db.Exec(`
CREATE TABLE department (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at datetime,
	updated_at datetime,
	deleted_at INTEGER NOT NULL DEFAULT 0,
	code TEXT NOT NULL DEFAULT '',
	is_system INTEGER NOT NULL DEFAULT 0,
	pid INTEGER NOT NULL DEFAULT 0,
	pids TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	level INTEGER NOT NULL DEFAULT 1,
	sort INTEGER NOT NULL DEFAULT 0,
	children_num INTEGER NOT NULL DEFAULT 0,
	user_number INTEGER NOT NULL DEFAULT 0
)`).Error; err != nil {
		return err
	}
	return db.Exec(`
CREATE TABLE role_data_scope_dept (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at datetime,
	updated_at datetime,
	role_id INTEGER NOT NULL DEFAULT 0,
	dept_id INTEGER NOT NULL DEFAULT 0
)`).Error
}
