package admin

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
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

	got := service.handleMutationError(businessErr, e.FAILURE)
	if got != businessErr {
		t.Fatalf("expected original business error, got %#v", got)
	}
}

func TestAdminUserHandleMutationErrorWrapsPlainError(t *testing.T) {
	service := NewAdminUserService()

	err := service.handleMutationError(errors.New("plain"), e.CreateUserFailed)
	assertBusinessErrorMessage(t, err, e.CreateUserFailed, "创建用户失败，请重试")
}

func TestRevokeUserTokensAfterCommitReturnsError(t *testing.T) {
	service := NewAdminUserService()
	wantErr := errors.New("revoke failed")
	previous := revokeUserTokensFunc
	revokeUserTokensFunc = func(tx *gorm.DB, userID uint, revokedCode uint8, revokedReason string) error {
		_ = tx
		if userID != 12 || revokedCode != model.RevokedCodePasswordChangeAdmin || revokedReason == "" {
			t.Fatalf("unexpected revocation input: userID=%d code=%d reason=%q", userID, revokedCode, revokedReason)
		}
		return wantErr
	}
	defer func() {
		revokeUserTokensFunc = previous
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
