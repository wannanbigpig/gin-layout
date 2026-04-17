package admin

import (
	"errors"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminUserBuildListCondition(t *testing.T) {
	status := uint8(1)
	params := &form.AdminUserList{
		UserName:    "root",
		ID:          7,
		NickName:    "admin",
		Email:       "a@example.com",
		PhoneNumber: "138",
		Status:      &status,
		DeptId:      3,
	}

	condition, args := NewAdminUserService().buildListCondition(params)
	expected := "username like ? AND id = ? AND nickname like ? AND email like ? AND full_phone_number like ? AND status = ? AND EXISTS (SELECT 1 FROM admin_user_department_map WHERE admin_user_department_map.uid = admin_user.id AND admin_user_department_map.dept_id = ?)"
	if condition != expected {
		t.Fatalf("unexpected condition: %s", condition)
	}
	if len(args) != 7 {
		t.Fatalf("unexpected args len: %d", len(args))
	}
}

func TestUniqueUintSlice(t *testing.T) {
	menuIDs := access.UniqueUintSlice([]uint{2, 5, 2, 0, 5})
	if len(menuIDs) != 3 {
		t.Fatalf("unexpected menu id count: %#v", menuIDs)
	}
	if menuIDs[0] != 2 || menuIDs[1] != 5 || menuIDs[2] != 0 {
		t.Fatalf("unexpected menu ids: %#v", menuIDs)
	}
}

func TestUserPermissionSyncUserKey(t *testing.T) {
	key := access.NewUserPermissionSyncService().UserKey(12)
	if key != "adminUser:12" {
		t.Fatalf("unexpected user key: %s", key)
	}
}

func TestAdminUserMenuQuery(t *testing.T) {
	service := NewAdminUserService()

	condition, args := service.userMenuQuery(true, nil)
	if condition != "status = ?" || len(args) != 1 {
		t.Fatalf("unexpected super admin query: %s %#v", condition, args)
	}

	condition, args = service.userMenuQuery(false, nil)
	if condition != "status = ? AND is_auth = ?" || len(args) != 2 {
		t.Fatalf("unexpected anonymous menu query: %s %#v", condition, args)
	}

	condition, args = service.userMenuQuery(false, []uint{1, 2})
	if condition != "status = ? AND (is_auth = ? OR (is_auth = ? AND id IN (?)))" || len(args) != 4 {
		t.Fatalf("unexpected scoped menu query: %s %#v", condition, args)
	}
}

func TestAdminUserHandleMutationErrorKeepsBusinessError(t *testing.T) {
	service := NewAdminUserService()
	businessErr := e.NewBusinessError(e.FAILURE, "business")

	got := service.handleMutationError(businessErr, "fallback")
	if got != businessErr {
		t.Fatalf("expected original business error, got %#v", got)
	}
}

func TestAdminUserHandleMutationErrorWrapsPlainError(t *testing.T) {
	service := NewAdminUserService()

	err := service.handleMutationError(errors.New("plain"), "fallback")
	assertBusinessErrorMessage(t, err, e.FAILURE, "fallback")
}

func TestAdminUserListOptionsDepartmentSelectFields(t *testing.T) {
	service := NewAdminUserService()
	options := service.adminUserListOptions()
	scope, ok := options.Preload["Department"]
	if !ok || scope == nil {
		t.Fatal("expected Department preload scope")
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("failed to create dry-run db: %v", err)
	}

	scopedDB := scope(db)
	selects := strings.Join(scopedDB.Statement.Selects, ",")
	for _, field := range []string{"id", "name", "pid"} {
		if !strings.Contains(selects, field) {
			t.Fatalf("expected preload select to include %q, got %q", field, selects)
		}
	}
}

func TestApplyUpdateFieldsDefaultsCountryCodeWhenPhoneChanges(t *testing.T) {
	service := NewAdminUserService()
	phoneNumber := "13800138000"
	adminUserModel := &model.AdminUser{
		CountryCode: "+1",
	}

	err := service.applyUpdateFields(adminUserModel, &adminUserEditParams{
		Id:          1,
		PhoneNumber: &phoneNumber,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if adminUserModel.CountryCode != global.ChinaCountryCode {
		t.Fatalf("expected default country code %q, got %q", global.ChinaCountryCode, adminUserModel.CountryCode)
	}
	if adminUserModel.FullPhoneNumber != global.ChinaCountryCode+phoneNumber {
		t.Fatalf("unexpected full phone number: %q", adminUserModel.FullPhoneNumber)
	}
}

func TestApplyUpdateFieldsRejectsSuperAdminPasswordChange(t *testing.T) {
	service := NewAdminUserService()
	password := "new-password"
	adminUserModel := &model.AdminUser{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			BaseModel: model.BaseModel{ID: global.SuperAdminId},
		},
		Password: "hashed-password",
	}

	err := service.applyUpdateFields(adminUserModel, &adminUserEditParams{
		Id:       global.SuperAdminId,
		Password: &password,
	})

	assertBusinessErrorMessage(t, err, e.SuperAdminCannotModify, "系统默认超级管理员不允许修改密码")
}
