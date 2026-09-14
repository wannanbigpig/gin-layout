package admin_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/tests"
)

var (
	router        *gin.Engine
	authorization string
	mysqlEnabled  bool
)

func TestMain(m *testing.M) {
	var err error
	router, err = tests.SetupRouter()
	if err != nil {
		_, _ = os.Stderr.WriteString("初始化测试路由失败: " + err.Error() + "\n")
		os.Exit(1)
	}
	db, err := model.GetDB()
	if err != nil {
		_, _ = os.Stderr.WriteString("获取真实 MySQL 连接失败: " + err.Error() + "\n")
		os.Exit(1)
	}
	adminUser := model.NewAdminUsers()
	if err := db.Where("id = ? AND deleted_at = 0", global.SuperAdminId).First(adminUser).Error; err != nil {
		_, _ = os.Stderr.WriteString("测试库缺少 ID=1 超级管理员，请先初始化系统数据: " + err.Error() + "\n")
		os.Exit(1)
	}
	if adminUser.IsSuperAdmin != global.Yes {
		_, _ = os.Stderr.WriteString("测试库 ID=1 用户不是超级管理员，请修复测试库初始数据\n")
		os.Exit(1)
	}
	claims := token.NewAdminCustomClaims(adminUser)
	accessToken, err := token.Generate(claims)
	if err != nil {
		_, _ = os.Stderr.WriteString("创建管理员Token失败: " + err.Error() + "\n")
		os.Exit(1)
	}
	authorization = "Bearer " + accessToken
	mysqlEnabled = true

	testSession := &model.AdminLoginLogs{
		UID:          adminUser.ID,
		Username:     adminUser.Username,
		JwtID:        claims.ID,
		UserAgent:    "go-layout-interface-test",
		LoginStatus:  model.LoginStatusSuccess,
		Type:         model.LoginTypeLogin,
		IsRevoked:    model.IsRevokedNo,
		TokenExpires: &utils.FormatDate{Time: claims.ExpiresAt.Time},
	}
	if err := db.Create(testSession).Error; err != nil {
		_, _ = os.Stderr.WriteString("创建接口测试认证会话失败: " + err.Error() + "\n")
		os.Exit(1)
	}

	exitCode := m.Run()
	if err := db.Unscoped().Delete(&model.AdminLoginLogs{}, testSession.ID).Error; err != nil {
		_, _ = os.Stderr.WriteString("清理接口测试认证会话失败: " + err.Error() + "\n")
		if exitCode == 0 {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func postRequest(route string, body *string) *httptest.ResponseRecorder {
	return performRequest(http.MethodPost, route, body, authorization)
}

func anonymousPostRequest(route string, body *string) *httptest.ResponseRecorder {
	return performRequest(http.MethodPost, route, body, "")
}

func getRequest(route string, queryParams *url.Values) *httptest.ResponseRecorder {
	path := route
	if queryParams != nil {
		path += "?" + queryParams.Encode()
	}
	return performRequest(http.MethodGet, path, nil, authorization)
}

func anonymousGetRequest(route string, queryParams *url.Values) *httptest.ResponseRecorder {
	path := route
	if queryParams != nil {
		path += "?" + queryParams.Encode()
	}
	return performRequest(http.MethodGet, path, nil, "")
}

func performRequest(method, route string, body *string, authHeader string) *httptest.ResponseRecorder {
	return performRequestWithHeaders(method, route, body, authHeader, nil)
}

func performRequestWithHeaders(method, route string, body *string, authHeader string, headers map[string]string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewBufferString(*body)
	}

	req := httptest.NewRequest(method, route, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// decodeResult 解析统一响应结构。
func decodeResult(t *testing.T, recorder *httptest.ResponseRecorder) *response.Result {
	t.Helper()

	result := new(response.Result)
	if err := json.Unmarshal(recorder.Body.Bytes(), result); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, recorder.Body.String())
	}
	return result
}

// requireMySQL 断言接口测试正在使用配置文件中的真实 MySQL。
func requireMySQL(t *testing.T) {
	t.Helper()
	if !mysqlEnabled {
		t.Fatal("接口测试必须在项目根 config.yaml 中启用真实 MySQL")
	}
}
