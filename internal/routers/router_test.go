package routers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	accesssvc "github.com/wannanbigpig/gin-layout/internal/service/access"
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

func TestAdminRouteTreeSeparatesPublicAuthenticatedAndPermissionRoutes(t *testing.T) {
	tree := AdminRouteTree(nil)
	if len(tree.Children) != 2 {
		t.Fatalf("expected public and authenticated route groups, got %d", len(tree.Children))
	}

	public := tree.Children[0]
	assertRouteGroupAuthMode(t, public, AuthModeNone)
	assertRouteGroupExcludesPaths(t, public,
		"/auth/refresh",
		"/auth/logout",
		"/common/ws",
	)

	authenticated := tree.Children[1]
	if len(authenticated.Children) != 2 {
		t.Fatalf("expected no-permission and permission route groups, got %d", len(authenticated.Children))
	}

	noPermission := authenticated.Children[0]
	assertRouteGroupContainsPaths(t, noPermission,
		"/auth/refresh",
		"/auth/logout",
		"/common/ws",
	)
	if len(noPermission.Children) != 3 {
		t.Fatalf("expected session credential, websocket ticket and access-token route groups, got %d", len(noPermission.Children))
	}
	assertRouteGroupAuthMode(t, noPermission.Children[2], AuthModeLogin)
	assertRouteGroupAuthMode(t, authenticated.Children[1], AuthModeAuth)
}

func assertRouteGroupContainsPaths(t *testing.T, group RouteGroupDef, expected ...string) {
	t.Helper()
	paths := collectRouteGroupPaths(group)
	for _, path := range expected {
		if !paths[path] {
			t.Errorf("expected route group to contain path %s", path)
		}
	}
}

func assertRouteGroupExcludesPaths(t *testing.T, group RouteGroupDef, excluded ...string) {
	t.Helper()
	paths := collectRouteGroupPaths(group)
	for _, path := range excluded {
		if paths[path] {
			t.Errorf("expected route group to exclude path %s", path)
		}
	}
}

func collectRouteGroupPaths(group RouteGroupDef) map[string]bool {
	routes := CollectRouteMeta(group)
	paths := make(map[string]bool, len(routes))
	for _, route := range routes {
		paths[route.Path] = true
	}
	return paths
}

func assertRouteGroupAuthMode(t *testing.T, group RouteGroupDef, expected AuthMode) {
	t.Helper()

	if group.Security == SecurityInherit {
		switch expected {
		case AuthModeLogin:
			group.Security = SecurityAccessToken
		case AuthModeAuth:
			group.Security = SecurityAccessTokenPermission
		default:
			group.Security = SecurityPublic
		}
	}
	catalog, err := CompileRouteTree(group)
	if err != nil {
		t.Fatalf("compile route group: %v", err)
	}
	routes := catalog.Routes()
	if len(routes) == 0 {
		t.Errorf("expected route group for auth mode %d to contain routes", expected)
	}
	for _, route := range routes {
		if route.Auth != expected {
			t.Errorf("route %s %s has auth mode %d, expected %d", route.Method, route.Path, route.Auth, expected)
		}
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

	commonUploadCode := utils.MD5(http.MethodPost + "_/admin/v1/common/upload")
	if route, ok := routes[commonUploadCode]; !ok || route.Auth != AuthModeLogin {
		t.Fatalf("missing or invalid common upload route: %#v", route)
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
	if route, ok := routes[notificationWSCode]; !ok || route.Auth != AuthModeNone || route.GroupCode != "common_ws" {
		t.Fatalf("missing or invalid notification ws route: %#v", route)
	}

	refreshCode := utils.MD5(http.MethodPost + "_/admin/v1/auth/refresh")
	if route, ok := routes[refreshCode]; !ok || route.Auth != AuthModeNone || route.GroupCode != "auth_public" {
		t.Fatalf("missing or invalid refresh route: %#v", route)
	}

	logoutCode := utils.MD5(http.MethodPost + "_/admin/v1/auth/logout")
	if route, ok := routes[logoutCode]; !ok || route.Auth != AuthModeNone || route.GroupCode != "auth_public" {
		t.Fatalf("missing or invalid logout route: %#v", route)
	}
}

func TestAuthModeAuthRoutesHaveDefaultMenuAPIBindingOrExemption(t *testing.T) {
	routes := CollectRouteMeta(AppRouteTree())
	bindings := accesssvc.DefaultMenuAPIBindingRouteSet()
	exemptions := map[string]string{}

	var missing []string
	for _, route := range routes {
		if route.Auth != AuthModeAuth {
			continue
		}
		key := route.Method + " " + route.Path
		if _, ok := bindings[key]; ok {
			continue
		}
		if _, ok := exemptions[key]; ok {
			continue
		}
		missing = append(missing, key)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("missing default menu API bindings or explicit exemptions:\n%s", strings.Join(missing, "\n"))
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

func TestSetRoutersGlobalRateLimitDefaultEnabled(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Mysql.Enable = false
		cfg.RateLimitEnabled = true
		cfg.RateLimitRPS = 1
		cfg.RateLimitBurst = 1
		cfg.LoginRateLimitEnabled = false
	})
	defer restoreConfig()

	engine, err := SetRouters()
	if err != nil {
		t.Fatalf("SetRouters returned error: %v", err)
	}

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodGet, "/ping", nil)
	firstReq.RemoteAddr = "10.10.0.1:1234"
	engine.ServeHTTP(first, firstReq)
	if first.Code != http.StatusOK {
		t.Fatalf("expected first request status %d, got %d", http.StatusOK, first.Code)
	}

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodGet, "/ping", nil)
	secondReq.RemoteAddr = "10.10.0.1:1234"
	engine.ServeHTTP(second, secondReq)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status %d, got %d", http.StatusTooManyRequests, second.Code)
	}
}

func TestLoginRouteUsesDedicatedRateLimit(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Mysql.Enable = false
		cfg.RateLimitEnabled = true
		cfg.RateLimitRPS = 100
		cfg.RateLimitBurst = 100
		cfg.LoginRateLimitEnabled = true
		cfg.LoginRateLimitRPS = 1
		cfg.LoginRateLimitBurst = 1
	})
	defer restoreConfig()

	engine, err := SetRouters()
	if err != nil {
		t.Fatalf("SetRouters returned error: %v", err)
	}

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/admin/v1/login", strings.NewReader(`{}`))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.RemoteAddr = "10.10.0.2:1234"
	engine.ServeHTTP(first, firstReq)
	if first.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected first login request status %d, got %d", http.StatusServiceUnavailable, first.Code)
	}

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/admin/v1/login", strings.NewReader(`{}`))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.RemoteAddr = "10.10.0.2:1234"
	engine.ServeHTTP(second, secondReq)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second login request status %d, got %d", http.StatusTooManyRequests, second.Code)
	}

	captcha := httptest.NewRecorder()
	captchaReq := httptest.NewRequest(http.MethodGet, "/admin/v1/login-captcha", nil)
	captchaReq.RemoteAddr = "10.10.0.2:1234"
	engine.ServeHTTP(captcha, captchaReq)
	if captcha.Code != http.StatusOK {
		t.Fatalf("expected captcha request status %d, got %d", http.StatusOK, captcha.Code)
	}
}

func TestLoginRateLimitCanBeDisabled(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Mysql.Enable = false
		cfg.RateLimitEnabled = false
		cfg.LoginRateLimitEnabled = false
		cfg.LoginRateLimitRPS = 1
		cfg.LoginRateLimitBurst = 1
	})
	defer restoreConfig()

	engine, err := SetRouters()
	if err != nil {
		t.Fatalf("SetRouters returned error: %v", err)
	}

	for i := 0; i < 3; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/v1/login", strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = "10.10.0.3:1234"
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected disabled login limiter request %d status %d, got %d", i, http.StatusServiceUnavailable, recorder.Code)
		}
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
