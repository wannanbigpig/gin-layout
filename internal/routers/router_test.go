package routers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

func TestSetRoutersRegistersApiMetadata(t *testing.T) {
	routes := CollectRouteMeta(AppRouteTree())

	menuCode := utils.MD5(http.MethodPost + "_/admin/v1/menu/update")
	menuRoute, ok := routes[menuCode]
	if !ok {
		t.Fatalf("missing route metadata for menu update")
	}
	if menuRoute.GroupCode != "menu" {
		t.Fatalf("unexpected group code: %s", menuRoute.GroupCode)
	}
	if menuRoute.Auth != AuthModeAuth {
		t.Fatalf("unexpected auth flag: %d", menuRoute.Auth)
	}
}

func TestSetRoutersRegistersPermissionCriticalRoutes(t *testing.T) {
	routes := CollectRouteMeta(AppRouteTree())

	dashboardStatisticsCode := utils.MD5(http.MethodGet + "_/admin/v1/dashboard/statistics")
	if route, ok := routes[dashboardStatisticsCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid dashboard statistics route: %#v", route)
	}

	checkTokenCode := utils.MD5(http.MethodGet + "_/admin/v1/auth/check-token")
	if route, ok := routes[checkTokenCode]; !ok || route.Auth != AuthModeLogin {
		t.Fatalf("missing or invalid auth check-token route: %#v", route)
	}

	updateProfileCode := utils.MD5(http.MethodPost + "_/admin/v1/admin-user/update-profile")
	if route, ok := routes[updateProfileCode]; !ok || route.Auth != AuthModeLogin {
		t.Fatalf("missing or invalid update-profile route: %#v", route)
	}

	fileCode := utils.MD5(http.MethodGet + "_/admin/v1/file/:uuid")
	if route, ok := routes[fileCode]; !ok || route.Auth != AuthModeNone {
		t.Fatalf("missing or invalid file route: %#v", route)
	}

	storageConfigCode := utils.MD5(http.MethodGet + "_/admin/v1/system/storage/config")
	if route, ok := routes[storageConfigCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid storage config route: %#v", route)
	}

	storageSecretCode := utils.MD5(http.MethodGet + "_/admin/v1/system/storage/secret")
	if route, ok := routes[storageSecretCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid storage secret route: %#v", route)
	}

	fileDestroyCode := utils.MD5(http.MethodPost + "_/admin/v1/system/file/trash/destroy")
	if route, ok := routes[fileDestroyCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid file destroy route: %#v", route)
	}

	fileBatchDeleteCode := utils.MD5(http.MethodPost + "_/admin/v1/system/file/batch-delete")
	if route, ok := routes[fileBatchDeleteCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid file batch-delete route: %#v", route)
	}

	credentialBatchCode := utils.MD5(http.MethodPost + "_/admin/v1/system/file/upload/credential/batch")
	if route, ok := routes[credentialBatchCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid credential batch route: %#v", route)
	}

	completeBatchCode := utils.MD5(http.MethodPost + "_/admin/v1/system/file/upload/complete/batch")
	if route, ok := routes[completeBatchCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid complete batch route: %#v", route)
	}

	multipartInitCode := utils.MD5(http.MethodPost + "_/admin/v1/system/file/upload/multipart/init")
	if route, ok := routes[multipartInitCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid multipart init route: %#v", route)
	}

	multipartCompleteCode := utils.MD5(http.MethodPost + "_/admin/v1/system/file/upload/multipart/complete")
	if route, ok := routes[multipartCompleteCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid multipart complete route: %#v", route)
	}

	multipartAbortCode := utils.MD5(http.MethodPost + "_/admin/v1/system/file/upload/multipart/abort")
	if route, ok := routes[multipartAbortCode]; !ok || route.Auth != AuthModeAuth {
		t.Fatalf("missing or invalid multipart abort route: %#v", route)
	}

	notificationListCode := utils.MD5(http.MethodGet + "_/admin/v1/system/notification/list")
	if route, ok := routes[notificationListCode]; !ok || route.Auth != AuthModeLogin {
		t.Fatalf("missing or invalid notification list route: %#v", route)
	}

	notificationWSCode := utils.MD5(http.MethodGet + "_/admin/v1/common/ws")
	if route, ok := routes[notificationWSCode]; !ok || route.Auth != AuthModeNone {
		t.Fatalf("missing or invalid notification ws route: %#v", route)
	}
}

func TestSetRoutersRegistersCriticalRoutes(t *testing.T) {
	engine, err := SetRouters()
	if err != nil {
		t.Fatalf("SetRouters returned error: %v", err)
	}
	routeMap := make(map[string]bool)
	for _, route := range engine.Routes() {
		routeMap[route.Method+" "+route.Path] = true
	}

	required := []string{
		http.MethodGet + " /ping",
		http.MethodGet + " /admin/v1/dashboard/overview",
		http.MethodGet + " /admin/v1/dashboard/statistics",
		http.MethodGet + " /admin/v1/admin-user/list",
		http.MethodPost + " /admin/v1/permission/update",
		http.MethodGet + " /admin/v1/menu/list",
		http.MethodGet + " /admin/v1/role/list",
		http.MethodGet + " /admin/v1/department/list",
		http.MethodGet + " /admin/v1/log/request/list",
		http.MethodGet + " /admin/v1/log/login/list",
		http.MethodGet + " /admin/v1/common/ws",
		http.MethodGet + " /admin/v1/system/notification/list",
		http.MethodGet + " /admin/v1/system/notification/unread-count",
		http.MethodPost + " /admin/v1/system/notification/read",
		http.MethodPost + " /admin/v1/system/notification/read-all",
		http.MethodGet + " /admin/v1/system/storage/secret",
		http.MethodPost + " /admin/v1/system/file/upload/credential/batch",
		http.MethodPost + " /admin/v1/system/file/upload/complete/batch",
		http.MethodPost + " /admin/v1/system/file/upload/multipart/init",
		http.MethodPost + " /admin/v1/system/file/upload/multipart/complete",
		http.MethodPost + " /admin/v1/system/file/upload/multipart/abort",
	}

	for _, route := range required {
		if !routeMap[route] {
			t.Fatalf("missing registered route: %s", route)
		}
	}
}

func TestLoginRouteReturnsDependencyNotReadyWhenMysqlUnavailable(t *testing.T) {
	restoreMysql := disableMysqlForRouterTest(t)
	defer restoreMysql()

	engine, err := SetRouters()
	if err != nil {
		t.Fatalf("SetRouters returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/v1/login", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}

	var result routeResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if result.Code != e.ServiceDependencyNotReady {
		t.Fatalf("expected code %d, got %d", e.ServiceDependencyNotReady, result.Code)
	}
}

func TestLoginCaptchaRouteRemainsAvailableWithoutMysql(t *testing.T) {
	restoreMysql := disableMysqlForRouterTest(t)
	defer restoreMysql()

	engine, err := SetRouters()
	if err != nil {
		t.Fatalf("SetRouters returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/v1/login-captcha", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var result routeResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal login captcha response: %v", err)
	}
	if result.Code != e.SUCCESS {
		t.Fatalf("expected code %d, got %d", e.SUCCESS, result.Code)
	}
}

func TestReadinessRouteReportsMysqlUnavailable(t *testing.T) {
	restoreState := disableDependenciesForReadinessTest(t)
	defer restoreState()

	engine, err := SetRouters()
	if err != nil {
		t.Fatalf("SetRouters returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health/readiness", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}

	var status readinessStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal readiness response: %v", err)
	}
	if status.Ready {
		t.Fatal("expected readiness to be false when mysql is unavailable")
	}
	if status.Dependencies.Mysql.Ready {
		t.Fatal("expected mysql readiness to be false")
	}
	if !status.Dependencies.Mysql.Required {
		t.Fatal("expected mysql to be marked as required")
	}
}

type routeResult struct {
	Code int `json:"code"`
}

func disableMysqlForRouterTest(t *testing.T) func() {
	t.Helper()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Mysql.Enable = false
	})
	if err := data.CloseMysql(); err != nil {
		t.Fatalf("close mysql: %v", err)
	}

	return func() {
		restoreConfig()
		if err := data.CloseMysql(); err != nil {
			t.Fatalf("close mysql: %v", err)
		}
	}
}

func disableDependenciesForReadinessTest(t *testing.T) func() {
	t.Helper()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Mysql.Enable = false
		cfg.Redis.Enable = false
		cfg.Queue.Enable = false
	})
	if err := data.CloseMysql(); err != nil {
		t.Fatalf("close mysql: %v", err)
	}
	if err := data.CloseRedis(); err != nil {
		t.Fatalf("close redis: %v", err)
	}

	return func() {
		restoreConfig()
		if err := data.CloseRedis(); err != nil {
			t.Fatalf("close redis: %v", err)
		}
		if err := data.CloseMysql(); err != nil {
			t.Fatalf("close mysql: %v", err)
		}
	}
}
