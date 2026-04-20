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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
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
	now := time.Now().UTC()
	expiresAt := now.Add(time.Second * c.Config.Jwt.TTL)
	claims := token.AdminCustomClaims{
		AdminUserInfo: token.AdminUserInfo{
			UserID:       1,
			Nickname:     "super_admin",
			IsSuperAdmin: 1,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    global.Issuer,
			Subject:   global.PcAdminSubject,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	accessToken, err := token.Generate(claims)
	if err != nil {
		_, _ = os.Stderr.WriteString("创建管理员Token失败: " + err.Error() + "\n")
		os.Exit(1)
	}
	authorization = "Bearer " + accessToken
	mysqlEnabled = c.Config.Mysql.Enable
	os.Exit(m.Run())
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

// requireMySQL 在需要真实数据库链路时跳过测试。
func requireMySQL(t *testing.T) {
	t.Helper()
	if !mysqlEnabled {
		t.Skip("当前测试配置未启用 MySQL，跳过需要真实数据库链路的接口流程测试")
	}
}
