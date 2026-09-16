package admin

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
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

type fakeAdminDataScopeFilter struct {
	cond       string
	args       []any
	scopeAll   bool
	deptIDs    []uint
	resolveErr error
}

func (f fakeAdminDataScopeFilter) Build(_ context.Context, _ access.DataScopeInput, deptCol, ownerCol string) (string, []any) {
	if f.cond != "" {
		return f.cond, f.args
	}
	return "(" + deptCol + " IN (?) OR " + ownerCol + " = ?)", []any{[]uint{20}, uint(3)}
}

func (f fakeAdminDataScopeFilter) ResolveDepartmentScope(_ context.Context, _ access.DataScopeInput) (bool, []uint, error) {
	return f.scopeAll, f.deptIDs, f.resolveErr
}

func TestAdminUserDataScopeConditionExecutesWithDepartmentMap(t *testing.T) {
	previous := newAdminDataScopeFilter
	newAdminDataScopeFilter = func() adminDataScopeFilter {
		return fakeAdminDataScopeFilter{}
	}
	defer func() {
		newAdminDataScopeFilter = previous
	}()

	service := NewAdminUserService()
	service.SetAdminUserId(9)
	condition, args := service.buildDataScopeCondition()
	expected := "(EXISTS (SELECT 1 FROM admin_user_department_map WHERE admin_user_department_map.uid = admin_user.id AND admin_user_department_map.dept_id IN (?)) OR admin_user.id = ?)"
	if condition != expected {
		t.Fatalf("unexpected data scope condition: %s", condition)
	}
	if !strings.Contains(condition, "admin_user_department_map.uid = admin_user.id") {
		t.Fatalf("expected condition to join through admin_user_department_map: %s", condition)
	}

	db := setupAdminUserDataScopeSQLiteDB(t)
	var ids []uint
	if err := db.Table("admin_user").
		Where(condition, args...).
		Order("id asc").
		Pluck("id", &ids).Error; err != nil {
		t.Fatalf("data scope condition should execute on sqlite: %v", err)
	}

	want := []uint{2, 3}
	if len(ids) != len(want) {
		t.Fatalf("unexpected scoped ids: got=%v want=%v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("unexpected scoped ids: got=%v want=%v", ids, want)
		}
	}
}

func TestAdminUserInDataScopeChecksTheTargetRow(t *testing.T) {
	db := setupAdminUserDataScopeSQLiteDB(t)
	condition := wrapAdminUserDeptScopeCondition("(admin_user_department_map.dept_id IN (?) OR admin_user.id = ?)")
	args := []any{[]uint{20}, uint(3)}

	tests := []struct {
		name   string
		userID uint
		want   bool
	}{
		{name: "rejects user outside department and owner scope", userID: 1, want: false},
		{name: "allows user in department scope", userID: 2, want: true},
		{name: "allows owner scope without department mapping", userID: 3, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := adminUserInDataScope(db, tt.userID, condition, args)
			if err != nil {
				t.Fatalf("check target data scope failed: %v", err)
			}
			if got != tt.want {
				t.Fatalf("adminUserInDataScope(%d) = %v, want %v", tt.userID, got, tt.want)
			}
		})
	}
}

func TestWrapAdminUserDeptScopeConditionKeepsOwnerOutsideExists(t *testing.T) {
	condition := wrapAdminUserDeptScopeCondition("(admin_user_department_map.dept_id IN (?) OR admin_user.id = ?)")
	expected := "(EXISTS (SELECT 1 FROM admin_user_department_map WHERE admin_user_department_map.uid = admin_user.id AND admin_user_department_map.dept_id IN (?)) OR admin_user.id = ?)"
	if condition != expected {
		t.Fatalf("unexpected wrapped condition: %s", condition)
	}
}

func TestAdminUserRequireDepartmentsInDataScope(t *testing.T) {
	tests := []struct {
		name      string
		filter    fakeAdminDataScopeFilter
		deptIDs   []uint
		wantCode  int
		wantError error
	}{
		{
			name:    "allows departments inside merged scope",
			filter:  fakeAdminDataScopeFilter{deptIDs: []uint{10, 20}},
			deptIDs: []uint{20, 20},
		},
		{
			name:     "rejects any department outside merged scope",
			filter:   fakeAdminDataScopeFilter{deptIDs: []uint{10, 20}},
			deptIDs:  []uint{20, 30},
			wantCode: e.AuthorizationErr,
		},
		{
			name:    "allows all departments for scope all",
			filter:  fakeAdminDataScopeFilter{scopeAll: true},
			deptIDs: []uint{999},
		},
		{
			name:      "returns scope resolution errors",
			filter:    fakeAdminDataScopeFilter{resolveErr: errors.New("scope unavailable")},
			deptIDs:   []uint{10},
			wantError: errors.New("scope unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := newAdminDataScopeFilter
			newAdminDataScopeFilter = func() adminDataScopeFilter { return tt.filter }
			defer func() { newAdminDataScopeFilter = previous }()

			service := NewAdminUserService()
			service.SetAdminUserId(9)
			err := service.requireDepartmentsInDataScope(tt.deptIDs)

			if tt.wantError != nil {
				if err == nil || err.Error() != tt.wantError.Error() {
					t.Fatalf("expected error %v, got %v", tt.wantError, err)
				}
				return
			}
			if tt.wantCode != 0 {
				var businessErr *e.BusinessError
				if !errors.As(err, &businessErr) || businessErr.GetCode() != tt.wantCode {
					t.Fatalf("expected business error code %d, got %v", tt.wantCode, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestAdminUserRequireDepartmentsInDataScopeAllowsEmptyAssignmentWithoutResolving(t *testing.T) {
	previous := newAdminDataScopeFilter
	newAdminDataScopeFilter = func() adminDataScopeFilter {
		return fakeAdminDataScopeFilter{resolveErr: errors.New("must not resolve empty assignment")}
	}
	defer func() { newAdminDataScopeFilter = previous }()

	if err := NewAdminUserService().requireDepartmentsInDataScope(nil); err != nil {
		t.Fatalf("empty department assignment should be allowed, got %v", err)
	}
}

func TestAdminUserMutationRejectsDepartmentOutsideScopeBeforeWrite(t *testing.T) {
	previous := newAdminDataScopeFilter
	newAdminDataScopeFilter = func() adminDataScopeFilter {
		return fakeAdminDataScopeFilter{deptIDs: []uint{10}}
	}
	defer func() { newAdminDataScopeFilter = previous }()

	deptIDs := []uint{20}
	service := NewAdminUserService()
	service.SetAdminUserId(9)
	err := service.saveAdminUserMutation(&adminUserEditParams{DeptIds: &deptIDs})

	assertBusinessErrorMessage(t, err, e.AuthorizationErr, "暂无权限")
}

func setupAdminUserDataScopeSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	statements := []string{
		`CREATE TABLE admin_user (
			id integer primary key,
			username text,
			deleted_at integer not null default 0
		)`,
		`CREATE TABLE admin_user_department_map (
			id integer primary key autoincrement,
			uid integer not null,
			dept_id integer not null
		)`,
		`INSERT INTO admin_user (id, username, deleted_at) VALUES
			(1, 'out-of-scope', 0),
			(2, 'dept-match', 0),
			(3, 'self-match-without-dept-map', 0)`,
		`INSERT INTO admin_user_department_map (uid, dept_id) VALUES
			(1, 10),
			(2, 20)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("execute sqlite setup failed: %v", err)
		}
	}
	return db
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

func TestGetUserMenuInfoTreatsIsSuperAdminAsFullAccess(t *testing.T) {
	db := setupAdminUserMenuInfoTestDB(t)
	adminUserModel := model.NewAdminUsers()
	adminUserModel.SetDB(db)
	menuModel := model.NewMenu()
	menuModel.SetDB(db)
	menuI18nModel := model.NewMenuI18n()
	menuI18nModel.SetDB(db)

	result, err := NewAdminUserService().getUserMenuInfoWithModels(2, "zh-CN", adminUserModel, menuModel, menuI18nModel)
	if err != nil {
		t.Fatalf("GetUserMenuInfo returned error: %v", err)
	}

	if !menuTreeContainsCode(result, "task:cancel") {
		t.Fatalf("expected is_super_admin user to receive task:cancel menu, got %#v", result)
	}
}

func TestAdminUserHandleMutationErrorKeepsBusinessError(t *testing.T) {
	service := NewAdminUserService()
	businessErr := e.NewBusinessError(e.FAILURE, "business")

	got := service.handleMutationError(businessErr, e.FAILURE)
	if got != businessErr {
		t.Fatalf("expected original business error, got %#v", got)
	}
}

func setupAdminUserMenuInfoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	statements := []string{
		`CREATE TABLE admin_user (
			id integer primary key,
			username text,
			is_super_admin integer not null default 0,
			deleted_at integer not null default 0
		)`,
		`CREATE TABLE menu (
			id integer primary key,
			created_at datetime,
			updated_at datetime,
			deleted_at integer not null default 0,
			icon text,
			code text,
			path text,
			full_path text,
			is_show integer,
			is_new_window integer,
			sort integer,
			type integer,
			pid integer,
			level integer,
			pids text,
			children_num integer,
			description text,
			is_auth integer,
			is_external_links integer,
			name text,
			component_key text,
			animate_enter text,
			animate_leave text,
			animate_duration real,
			status integer,
			redirect text
		)`,
		`CREATE TABLE menu_i18n (
			id integer primary key autoincrement,
			menu_id integer,
			locale text,
			title text
		)`,
		`INSERT INTO admin_user (id, username, is_super_admin, deleted_at) VALUES (2, 'ops-admin', 1, 0)`,
		`INSERT INTO menu (id, deleted_at, code, path, full_path, is_show, is_new_window, sort, type, pid, level, pids, children_num, is_auth, is_external_links, name, component_key, animate_duration, status)
			VALUES (10, 0, 'task', 'task', '/task', 1, 0, 100, 2, 0, 1, '0', 1, 1, 0, 'Task', 'system:task', 0, 1)`,
		`INSERT INTO menu (id, deleted_at, code, path, full_path, is_show, is_new_window, sort, type, pid, level, pids, children_num, is_auth, is_external_links, name, component_key, animate_duration, status)
			VALUES (11, 0, 'task:cancel', '', '', 1, 0, 90, 3, 10, 2, '0,10', 0, 1, 0, '', '', 0, 1)`,
		`INSERT INTO menu_i18n (menu_id, locale, title) VALUES (10, 'zh-CN', '任务中心'), (11, 'zh-CN', '取消任务')`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("execute sqlite setup failed: %v", err)
		}
	}
	return db
}

func menuTreeContainsCode(value any, code string) bool {
	switch tree := value.(type) {
	case []*resources.MenuCollectionResources:
		for _, item := range tree {
			if menuNodeContainsCode(item, code) {
				return true
			}
		}
	}
	return false
}

func menuNodeContainsCode(item *resources.MenuCollectionResources, code string) bool {
	if item == nil {
		return false
	}
	if item.Code == code {
		return true
	}
	for _, child := range item.Children {
		if menuNodeContainsCode(child, code) {
			return true
		}
	}
	return false
}

func TestAdminUserHandleMutationErrorWrapsPlainError(t *testing.T) {
	service := NewAdminUserService()

	err := service.handleMutationError(errors.New("plain"), e.CreateUserFailed)
	assertBusinessErrorMessage(t, err, e.CreateUserFailed, "创建用户失败，请重试")
}

func TestRevokeUserTokensAfterCommitReturnsError(t *testing.T) {
	service := NewAdminUserService()
	wantErr := errors.New("revoke failed")
	previous := completeUserTokenRevocationFunc
	completeUserTokenRevocationFunc = func(*auth.PreparedUserTokenRevocation) error {
		return wantErr
	}
	defer func() {
		completeUserTokenRevocationFunc = previous
	}()

	err := service.revokeUserTokensAfterCommit([]userTokenRevocation{{
		userID:        12,
		revokedCode:   model.RevokedCodePasswordChangeAdmin,
		revokedReason: "管理员修改密码",
	}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected revoke error %v, got %v", wantErr, err)
	}
}

func TestPrepareUserTokenRevocationUsesSecurityChangeReason(t *testing.T) {
	service := NewAdminUserService()
	previous := prepareUserTokenRevocationFunc
	prepareUserTokenRevocationFunc = func(_ *gorm.DB, userID uint, revokedCode uint8, revokedReason string) (*auth.PreparedUserTokenRevocation, error) {
		if userID != 12 || revokedCode != model.RevokedCodePasswordChangeAdmin || revokedReason != "管理员修改密码" {
			t.Fatalf("unexpected revocation input: userID=%d code=%d reason=%q", userID, revokedCode, revokedReason)
		}
		return &auth.PreparedUserTokenRevocation{}, nil
	}
	defer func() { prepareUserTokenRevocationFunc = previous }()

	item := &userTokenRevocation{userID: 12, revokedCode: model.RevokedCodePasswordChangeAdmin, revokedReason: "管理员修改密码"}
	if err := service.prepareUserTokenRevocation(nil, item); err != nil {
		t.Fatalf("prepare revocation failed: %v", err)
	}
	if item.prepared == nil {
		t.Fatal("expected prepared revocation to be retained until commit")
	}
}

func TestAdminUserUpdateValuesOnlyIncludesRequestedColumns(t *testing.T) {
	nickname := "new nickname"
	user := &model.AdminUser{
		Nickname:        nickname,
		Password:        "existing-hash",
		Status:          model.AdminUserStatusEnabled,
		IsSuperAdmin:    global.Yes,
		LastIp:          "127.0.0.1",
		FullPhoneNumber: "+8613800138000",
	}

	values := adminUserUpdateValues(user, &adminUserEditParams{Nickname: &nickname})
	if len(values) != 1 || values["nickname"] != nickname {
		t.Fatalf("expected nickname-only update, got %#v", values)
	}
	for _, field := range []string{"password", "status", "is_super_admin", "last_ip", "last_login", "full_phone_number"} {
		if _, exists := values[field]; exists {
			t.Fatalf("unrequested security/session column %q must not be overwritten: %#v", field, values)
		}
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE admin_user (
		id integer primary key,
		nickname text,
		password text,
		status integer,
		is_super_admin integer,
		last_ip text,
		last_login datetime,
		full_phone_number text,
		updated_at datetime,
		deleted_at integer default 0
	)`).Error; err != nil {
		t.Fatalf("create admin_user failed: %v", err)
	}
	if err := db.Exec(`INSERT INTO admin_user
		(id, nickname, password, status, is_super_admin, last_ip, last_login, full_phone_number, deleted_at)
		VALUES (12, 'old nickname', 'current-hash', 0, 1, '10.0.0.8', '2026-09-16 10:00:00', '+85255550000', 0)`).Error; err != nil {
		t.Fatalf("insert admin_user failed: %v", err)
	}
	if err := db.Model(&model.AdminUser{}).Where("id = ? AND deleted_at = 0", 12).Updates(values).Error; err != nil {
		t.Fatalf("apply exact-column update failed: %v", err)
	}
	var got struct {
		Nickname        string
		Password        string
		Status          uint8
		IsSuperAdmin    uint8
		LastIP          string `gorm:"column:last_ip"`
		LastLogin       string
		FullPhoneNumber string
	}
	if err := db.Table("admin_user").Where("id = ?", 12).Take(&got).Error; err != nil {
		t.Fatalf("query updated admin_user failed: %v", err)
	}
	if got.Nickname != nickname || got.Password != "current-hash" || got.Status != model.AdminUserStatusDisabled ||
		got.IsSuperAdmin != global.Yes || got.LastIP != "10.0.0.8" || got.FullPhoneNumber != "+85255550000" || got.LastLogin == "" {
		t.Fatalf("unexpected row after exact-column update: %#v", got)
	}
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
	// 记录本身尚无国家区号：仅改手机号时默认中国区号。
	adminUserModel := &model.AdminUser{}

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

func TestApplyUpdateFieldsPreservesExistingCountryCodeWhenPhoneChanges(t *testing.T) {
	service := NewAdminUserService()
	phoneNumber := "13800138000"
	// 记录已有非中国区号：仅改手机号、未传 country_code 时应保留既有区号，而不是被改成中国。
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
	if adminUserModel.CountryCode != "+1" {
		t.Fatalf("expected preserved country code %q, got %q", "+1", adminUserModel.CountryCode)
	}
	if adminUserModel.FullPhoneNumber != "+1"+phoneNumber {
		t.Fatalf("unexpected full phone number: %q", adminUserModel.FullPhoneNumber)
	}
}

func TestApplyUpdateFieldsClearsFullPhoneWhenPhoneCleared(t *testing.T) {
	service := NewAdminUserService()
	phoneNumber := ""
	adminUserModel := &model.AdminUser{
		CountryCode:     "+86",
		PhoneNumber:     "13800138000",
		FullPhoneNumber: "+8613800138000",
	}

	err := service.applyUpdateFields(adminUserModel, &adminUserEditParams{
		Id:          1,
		PhoneNumber: &phoneNumber,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if adminUserModel.PhoneNumber != "" {
		t.Fatalf("expected phone number to be cleared, got %q", adminUserModel.PhoneNumber)
	}
	if adminUserModel.FullPhoneNumber != "" {
		t.Fatalf("expected full phone number to be cleared, got %q", adminUserModel.FullPhoneNumber)
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

func TestApplyCreateFieldsSetsIsSuperAdmin(t *testing.T) {
	service := NewAdminUserService()
	username := "ops_admin"
	nickname := "Ops Admin"
	password := "password123"
	isSuperAdmin := global.Yes
	adminUserModel := &model.AdminUser{}

	err := service.applyCreateFields(adminUserModel, &adminUserEditParams{
		Username:     &username,
		Nickname:     &nickname,
		Password:     &password,
		IsSuperAdmin: &isSuperAdmin,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if adminUserModel.IsSuperAdmin != global.Yes {
		t.Fatalf("expected is_super_admin=%d, got %d", global.Yes, adminUserModel.IsSuperAdmin)
	}
}

func TestApplyUpdateFieldsSetsIsSuperAdmin(t *testing.T) {
	service := NewAdminUserService()
	isSuperAdmin := global.Yes
	adminUserModel := &model.AdminUser{}

	err := service.applyUpdateFields(adminUserModel, &adminUserEditParams{
		Id:           2,
		IsSuperAdmin: &isSuperAdmin,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if adminUserModel.IsSuperAdmin != global.Yes {
		t.Fatalf("expected is_super_admin=%d, got %d", global.Yes, adminUserModel.IsSuperAdmin)
	}
}

func TestApplyUpdateFieldsRejectsDefaultSuperAdminFlagRemoval(t *testing.T) {
	service := NewAdminUserService()
	isSuperAdmin := global.No
	adminUserModel := &model.AdminUser{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			BaseModel: model.BaseModel{ID: global.SuperAdminId},
		},
		IsSuperAdmin: global.Yes,
	}

	err := service.applyUpdateFields(adminUserModel, &adminUserEditParams{
		Id:           global.SuperAdminId,
		IsSuperAdmin: &isSuperAdmin,
	})

	assertBusinessErrorMessage(t, err, e.SuperAdminCannotModify, "系统默认超级管理员不允许修改密码")
}

func TestAdminUserMutationRejectsSuperAdminFlagChangeByNonSuperAdmin(t *testing.T) {
	service := NewAdminUserService()
	service.SetAdminUserId(2)
	isSuperAdmin := global.Yes

	restoreLoader := replaceSuperAdminCheckLoader(t, &model.AdminUser{IsSuperAdmin: global.No}, nil)
	defer restoreLoader()

	err := service.saveAdminUserMutation(&adminUserEditParams{
		Username:     stringPtr("ops_admin"),
		Nickname:     stringPtr("Ops Admin"),
		Password:     stringPtr("password123"),
		IsSuperAdmin: &isSuperAdmin,
	})

	assertBusinessErrorMessage(t, err, e.AuthorizationErr, "暂无权限")
}

func TestAdminUserMutationAllowsSuperAdminFlagChangeBySuperAdmin(t *testing.T) {
	service := NewAdminUserService()
	service.SetAdminUserId(2)
	isSuperAdmin := global.Yes

	restoreLoader := replaceSuperAdminCheckLoader(t, &model.AdminUser{IsSuperAdmin: global.Yes}, nil)
	defer restoreLoader()

	err := service.saveAdminUserMutation(&adminUserEditParams{
		Username:     stringPtr("ops_admin"),
		Nickname:     stringPtr("Ops Admin"),
		Password:     stringPtr("password123"),
		IsSuperAdmin: &isSuperAdmin,
	})

	if err == nil {
		t.Fatal("expected later DB error after super admin authorization passes")
	}
	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) && businessErr.GetCode() == e.AuthorizationErr {
		t.Fatalf("expected super admin actor not to be rejected by authorization, got %v", err)
	}
}

func replaceSuperAdminCheckLoader(t *testing.T, user *model.AdminUser, err error) func() {
	t.Helper()
	previous := loadAdminUserForSuperAdminCheck
	loadAdminUserForSuperAdminCheck = func(uint) (*model.AdminUser, error) {
		return user, err
	}
	return func() {
		loadAdminUserForSuperAdminCheck = previous
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestValidateAvatarFileAllowsRealJPGPNGAndWebP(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  []byte
	}{
		{name: "jpeg content with misleading gif extension", filename: "avatar.gif", content: testJPEGContent()},
		{name: "png content", filename: "avatar.png", content: testPNGContent()},
		{name: "webp content", filename: "avatar.txt", content: testWebPContent()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := newAdminMultipartFileHeader(t, tt.filename, tt.content)

			if err := validateAvatarFile(header); err != nil {
				t.Fatalf("expected avatar to pass validation, got %v", err)
			}
		})
	}
}

func TestValidateAvatarFileRejectsOversizeAvatar(t *testing.T) {
	content := append(testJPEGContent(), bytes.Repeat([]byte{0}, int(maxAvatarSize)+1)...)
	header := newAdminMultipartFileHeader(t, "avatar.jpg", content)

	err := validateAvatarFile(header)

	assertBusinessErrorMessage(t, err, e.InvalidParameter, "头像大小不能大于2MB")
}

func TestValidateAvatarFileRejectsGIFAndNonImage(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  []byte
	}{
		{name: "gif content", filename: "avatar.jpg", content: []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00")},
		{name: "plain text", filename: "avatar.png", content: []byte("not an image")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := newAdminMultipartFileHeader(t, tt.filename, tt.content)

			err := validateAvatarFile(header)

			assertBusinessErrorMessage(t, err, e.InvalidParameter, "仅支持jpg/jpeg/png/webp头像")
		})
	}
}

func newAdminMultipartFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file failed: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/", body)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(int64(len(content) + 1024)); err != nil {
		t.Fatalf("parse multipart form failed: %v", err)
	}
	return req.MultipartForm.File["file"][0]
}

func testJPEGContent() []byte {
	return []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00}
}

func testPNGContent() []byte {
	return []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}
}

func testWebPContent() []byte {
	return []byte{'R', 'I', 'F', 'F', 0x10, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' ', 0x00, 0x00, 0x00, 0x00}
}
