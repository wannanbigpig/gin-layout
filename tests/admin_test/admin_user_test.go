package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
)

func TestGetAdminUserRequiresLogin(t *testing.T) {
	queryParams := &url.Values{}
	queryParams.Set("id", "1")
	resp := anonymousGetRequest("/admin/v1/admin-user/get", queryParams)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestGetCurrentAdminUserWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/admin-user/get", nil)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)

	data, ok := result.Data.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, float64(1), data["id"])
}

func TestGetUserMenuInfoWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/admin-user/user-menu-info", nil)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestUpdateProfileInvalidEmail(t *testing.T) {
	requireMySQL(t)

	body := `{"email":"invalid-email"}`
	resp := postRequest("/admin/v1/admin-user/update-profile", &body)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.InvalidParameter, result.Code)
}

func TestAdminUserListWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/admin-user/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestAdminUserOptionsWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/admin-user/options", &url.Values{"keyword": {"super"}})
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, resp).Code)
}

func TestAdminUserListRequiresLogin(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/admin-user/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestAdminUserProtectedGetRoutesRequireLogin(t *testing.T) {
	testCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{name: "管理员详情需要登录", route: "/admin/v1/admin-user/detail", query: &url.Values{"id": {"1"}}},
		{name: "管理员完整手机号需要登录", route: "/admin/v1/admin-user/get-full-phone", query: &url.Values{"id": {"1"}}},
		{name: "管理员完整邮箱需要登录", route: "/admin/v1/admin-user/get-full-email", query: &url.Values{"id": {"1"}}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := anonymousGetRequest(tc.route, tc.query)

			assert.Equal(t, http.StatusUnauthorized, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.NotLogin, result.Code)
		})
	}
}

func TestAdminUserProtectedPostRoutesRequireLogin(t *testing.T) {
	testCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "管理员创建需要登录", route: "/admin/v1/admin-user/create", body: `{}`},
		{name: "管理员更新需要登录", route: "/admin/v1/admin-user/update", body: `{}`},
		{name: "管理员删除需要登录", route: "/admin/v1/admin-user/delete", body: `{}`},
		{name: "管理员绑定角色需要登录", route: "/admin/v1/admin-user/bind-role", body: `{}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			resp := anonymousPostRequest(tc.route, &body)

			assert.Equal(t, http.StatusUnauthorized, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.NotLogin, result.Code)
		})
	}
}

func TestAdminUserWriteFlow(t *testing.T) {
	requireWritableDB(t)

	username := fmt.Sprintf("ta%d", time.Now().UnixNano()%1e10)
	cleanupAdminUsers(t, "ta")

	createBody := map[string]any{
		"username": username,
		"nickname": "测试管理员",
		"password": "12345678",
		"email":    username + "@example.com",
		"dept_ids": []uint{1},
		"status":   1,
	}
	bodyBytes, _ := json.Marshal(createBody)
	body := string(bodyBytes)

	resp := postRequest("/admin/v1/admin-user/create", &body)
	result := decodeResult(t, resp)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, e.SUCCESS, result.Code)

	user := findAdminUserByUsername(t, username)

	detailResp := getRequest("/admin/v1/admin-user/detail", &url.Values{"id": {strconv.FormatUint(uint64(user.ID), 10)}})
	detailResult := decodeResult(t, detailResp)
	assert.Equal(t, e.SUCCESS, detailResult.Code)

	fullPhoneResp := getRequest("/admin/v1/admin-user/get-full-phone", &url.Values{"id": {strconv.FormatUint(uint64(user.ID), 10)}})
	assert.Equal(t, http.StatusOK, fullPhoneResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, fullPhoneResp).Code)

	fullEmailResp := getRequest("/admin/v1/admin-user/get-full-email", &url.Values{"id": {strconv.FormatUint(uint64(user.ID), 10)}})
	assert.Equal(t, http.StatusOK, fullEmailResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, fullEmailResp).Code)

	roleID := firstActiveRoleID(t)
	bindRolePayload := mustJSONBody(t, map[string]any{"user_id": user.ID, "role_ids": []uint{roleID}})
	bindRoleResp := postRequest("/admin/v1/admin-user/bind-role", &bindRolePayload)
	assert.Equal(t, http.StatusOK, bindRoleResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, bindRoleResp).Code)
	boundRoleIDs, err := model.NewAdminUserRoleMap().RoleIdsByUid(user.ID)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []uint{roleID}, boundRoleIDs)

	updateBody := map[string]any{
		"id":       user.ID,
		"nickname": "测试管理员-更新",
		"email":    username + "-updated@example.com",
		"dept_ids": []uint{1},
	}
	updateBytes, _ := json.Marshal(updateBody)
	updatePayload := string(updateBytes)
	updateResp := postRequest("/admin/v1/admin-user/update", &updatePayload)
	updateResult := decodeResult(t, updateResp)
	assert.Equal(t, e.SUCCESS, updateResult.Code)

	deleteBytes, _ := json.Marshal(map[string]any{"id": user.ID})
	deletePayload := string(deleteBytes)
	deleteResp := postRequest("/admin/v1/admin-user/delete", &deletePayload)
	deleteResult := decodeResult(t, deleteResp)
	assert.Equal(t, e.SUCCESS, deleteResult.Code)
}

func TestAdminUserDataScopeProtectsDetailAndMutations(t *testing.T) {
	requireWritableDB(t)
	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取测试数据库失败: %v", err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	insideDept := &model.Department{Code: "ds-in-" + suffix, Name: "DataScope 内-" + suffix, Pids: "0", Level: 1}
	outsideDept := &model.Department{Code: "ds-out-" + suffix, Name: "DataScope 外-" + suffix, Pids: "0", Level: 1}
	if err := db.Create([]*model.Department{insideDept, outsideDept}).Error; err != nil {
		t.Fatalf("创建 DataScope 测试部门失败: %v", err)
	}

	role := &model.Role{
		Code:      "ds-role-" + suffix,
		Name:      "DataScope 角色-" + suffix,
		Status:    1,
		DataScope: access.DataScopeDept,
	}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("创建 DataScope 测试角色失败: %v", err)
	}

	actor := &model.AdminUser{Username: "ds-actor-" + suffix, Nickname: "DataScope 操作人", Status: 1, IsSuperAdmin: global.No, CountryCode: "86"}
	insideUser := &model.AdminUser{Username: "ds-inside-" + suffix, Nickname: "DataScope 范围内", Status: 1, IsSuperAdmin: global.No, CountryCode: "86"}
	outsideUser := &model.AdminUser{Username: "ds-outside-" + suffix, Nickname: "DataScope 范围外", Status: 1, IsSuperAdmin: global.No, CountryCode: "86"}
	users := []*model.AdminUser{actor, insideUser, outsideUser}
	if err := db.Create(users).Error; err != nil {
		t.Fatalf("创建 DataScope 测试用户失败: %v", err)
	}

	var sessionID uint
	t.Cleanup(func() {
		_ = access.NewPermissionSyncCoordinator().ClearUser(actor.ID)
		if sessionID > 0 {
			_ = db.Unscoped().Delete(&model.AdminLoginLogs{}, sessionID).Error
		}
		userIDs := []uint{actor.ID, insideUser.ID, outsideUser.ID}
		_ = db.Where("uid IN ?", userIDs).Delete(&model.AdminUserRoleMap{}).Error
		_ = db.Where("uid IN ?", userIDs).Delete(&model.AdminUserDeptMap{}).Error
		_ = db.Where("role_id = ?", role.ID).Delete(&model.RoleMenuMap{}).Error
		_ = db.Unscoped().Where("id IN ?", userIDs).Delete(&model.AdminUser{}).Error
		_ = db.Unscoped().Delete(&model.Role{}, role.ID).Error
		_ = db.Unscoped().Where("id IN ?", []uint{insideDept.ID, outsideDept.ID}).Delete(&model.Department{}).Error
	})

	if err := db.Create(&[]model.AdminUserDeptMap{
		{Uid: actor.ID, DeptId: insideDept.ID},
		{Uid: insideUser.ID, DeptId: insideDept.ID},
		{Uid: outsideUser.ID, DeptId: outsideDept.ID},
	}).Error; err != nil {
		t.Fatalf("创建用户部门关系失败: %v", err)
	}
	if err := db.Create(&model.AdminUserRoleMap{Uid: actor.ID, RoleId: role.ID}).Error; err != nil {
		t.Fatalf("创建用户角色关系失败: %v", err)
	}
	permissionCodes := []string{"adminUser:list", "adminUser:update", "adminUser:bindRole", "adminUser:delete"}
	menuMappings := make([]model.RoleMenuMap, 0, len(permissionCodes))
	for _, code := range permissionCodes {
		menuMappings = append(menuMappings, model.RoleMenuMap{RoleId: role.ID, MenuId: findMenuByCode(t, code).ID})
	}
	if err := db.Create(&menuMappings).Error; err != nil {
		t.Fatalf("创建角色菜单关系失败: %v", err)
	}
	if err := access.NewPermissionSyncCoordinator().SyncUser(actor.ID); err != nil {
		t.Fatalf("同步 DataScope 测试用户权限失败: %v", err)
	}

	claims := token.NewAdminCustomClaims(actor)
	accessToken, err := token.Generate(claims)
	if err != nil {
		t.Fatalf("生成 DataScope 测试 token 失败: %v", err)
	}
	session := &model.AdminLoginLogs{
		UID:          actor.ID,
		Username:     actor.Username,
		JwtID:        claims.ID,
		UserAgent:    "go-layout-data-scope-test",
		LoginStatus:  model.LoginStatusSuccess,
		Type:         model.LoginTypeLogin,
		IsRevoked:    model.IsRevokedNo,
		TokenExpires: &utils.FormatDate{Time: claims.ExpiresAt.Time},
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("创建 DataScope 测试登录会话失败: %v", err)
	}
	sessionID = session.ID
	actorAuthorization := "Bearer " + accessToken

	insideDetail := performRequest(http.MethodGet, "/admin/v1/admin-user/detail?id="+strconv.FormatUint(uint64(insideUser.ID), 10), nil, actorAuthorization)
	assert.Equal(t, http.StatusOK, insideDetail.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, insideDetail).Code)

	outsideID := strconv.FormatUint(uint64(outsideUser.ID), 10)
	for _, route := range []string{
		"/admin/v1/admin-user/detail?id=" + outsideID,
		"/admin/v1/admin-user/get-full-phone?id=" + outsideID,
		"/admin/v1/admin-user/get-full-email?id=" + outsideID,
	} {
		resp := performRequest(http.MethodGet, route, nil, actorAuthorization)
		assert.Equal(t, http.StatusForbidden, resp.Code, route)
		assert.Equal(t, e.AuthorizationErr, decodeResult(t, resp).Code, route)
	}

	updateBody := mustJSONBody(t, map[string]any{"id": outsideUser.ID, "nickname": "should-not-update"})
	updateResp := performRequest(http.MethodPost, "/admin/v1/admin-user/update", &updateBody, actorAuthorization)
	assert.Equal(t, http.StatusForbidden, updateResp.Code)
	assert.Equal(t, e.AuthorizationErr, decodeResult(t, updateResp).Code)

	bindRoleBody := mustJSONBody(t, map[string]any{"user_id": outsideUser.ID, "role_ids": []uint{role.ID}})
	bindRoleResp := performRequest(http.MethodPost, "/admin/v1/admin-user/bind-role", &bindRoleBody, actorAuthorization)
	assert.Equal(t, http.StatusForbidden, bindRoleResp.Code)
	assert.Equal(t, e.AuthorizationErr, decodeResult(t, bindRoleResp).Code)

	deleteBody := mustJSONBody(t, map[string]any{"id": outsideUser.ID})
	deleteResp := performRequest(http.MethodPost, "/admin/v1/admin-user/delete", &deleteBody, actorAuthorization)
	assert.Equal(t, http.StatusForbidden, deleteResp.Code)
	assert.Equal(t, e.AuthorizationErr, decodeResult(t, deleteResp).Code)

	var unchanged model.AdminUser
	if err := db.Where("id = ? AND deleted_at = 0", outsideUser.ID).First(&unchanged).Error; err != nil {
		t.Fatalf("范围外用户不应被删除: %v", err)
	}
	assert.Equal(t, "DataScope 范围外", unchanged.Nickname)
	roleIDs, err := model.NewAdminUserRoleMap().RoleIdsByUid(outsideUser.ID)
	assert.NoError(t, err)
	assert.Empty(t, roleIDs)
}
