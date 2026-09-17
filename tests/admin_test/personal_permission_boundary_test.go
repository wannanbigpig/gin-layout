package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
)

func TestPersonalDashboardAndLogPermissionBoundaryForRegularUser(t *testing.T) {
	requireWritableDB(t)
	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取测试数据库失败: %v", err)
	}

	user := &model.AdminUser{
		Username:     uniqueCompactTestName("personal-boundary"),
		Nickname:     "个人工作台权限测试用户",
		Status:       global.Yes,
		IsSuperAdmin: global.No,
		CountryCode:  "86",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("创建普通用户失败: %v", err)
	}

	claims := token.NewAdminCustomClaims(user)
	accessToken, err := token.Generate(claims)
	if err != nil {
		t.Fatalf("生成普通用户 Token 失败: %v", err)
	}
	session := &model.AdminLoginLogs{
		UID: user.ID, Username: user.Username, JwtID: claims.ID,
		UserAgent:   "go-layout-personal-permission-test",
		LoginStatus: model.LoginStatusSuccess, Type: model.LoginTypeLogin,
		IsRevoked:    model.IsRevokedNo,
		TokenExpires: &utils.FormatDate{Time: claims.ExpiresAt.Time},
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("创建普通用户会话失败: %v", err)
	}
	t.Cleanup(func() {
		_ = access.NewPermissionSyncCoordinator().ClearUser(user.ID)
		_ = db.Unscoped().Delete(&model.AdminLoginLogs{}, session.ID).Error
		_ = db.Unscoped().Delete(&model.AdminUser{}, user.ID).Error
	})

	authorization := "Bearer " + accessToken
	for _, route := range []string{
		"/admin/v1/dashboard/personal",
		"/admin/v1/log/request/self-list?page=1&per_page=5&operator_id=1",
	} {
		resp := performRequest(http.MethodGet, route, nil, authorization)
		assert.Equal(t, http.StatusOK, resp.Code, route)
		assert.Equal(t, e.SUCCESS, decodeResult(t, resp).Code, route)
	}

	for _, route := range []string{
		"/admin/v1/dashboard/overview",
		"/admin/v1/dashboard/statistics",
		"/admin/v1/dashboard/monitor",
		"/admin/v1/log/request/list?page=1&per_page=5",
		"/admin/v1/log/login/list?page=1&per_page=5",
	} {
		resp := performRequest(http.MethodGet, route, nil, authorization)
		assert.Equal(t, http.StatusForbidden, resp.Code, route)
		assert.Equal(t, e.AuthorizationErr, decodeResult(t, resp).Code, route)
	}
}
