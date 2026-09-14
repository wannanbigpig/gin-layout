package admin_test

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/routers"
)

// TestAllProtectedRoutesRejectAnonymousRequests 直接从路由树生成用例，保证新增接口不会漏掉最基本的鉴权边界测试。
func TestAllProtectedRoutesRejectAnonymousRequests(t *testing.T) {
	routes := sortedAdminRoutes()
	for _, route := range routes {
		if route.Auth == routers.AuthModeNone {
			continue
		}
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			body := requestBodyForMethod(route.Method)
			resp := performRequest(route.Method, concreteRoutePath(route.Path), body, "")
			if resp.Code != http.StatusUnauthorized {
				t.Fatalf("受保护接口匿名请求未返回 401: http=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}
}

// TestAllRoutesWithoutUnifiedLoginHaveExplicitScenarios 要求每个不走统一登录中间件的管理端接口都有显式状态断言。
func TestAllRoutesWithoutUnifiedLoginHaveExplicitScenarios(t *testing.T) {
	type scenario struct {
		body       *string
		httpStatus int
	}
	emptyBody := `{}`
	scenarios := map[string]scenario{
		http.MethodGet + " /admin/v1/demo":          {httpStatus: http.StatusOK},
		http.MethodGet + " /admin/v1/file/:uuid":    {httpStatus: http.StatusBadRequest},
		http.MethodPost + " /admin/v1/login":        {body: &emptyBody, httpStatus: http.StatusBadRequest},
		http.MethodGet + " /admin/v1/login-captcha": {httpStatus: http.StatusOK},
		http.MethodPost + " /admin/v1/auth/refresh": {body: &emptyBody, httpStatus: http.StatusForbidden},
		http.MethodPost + " /admin/v1/auth/logout":  {body: &emptyBody, httpStatus: http.StatusForbidden},
		http.MethodGet + " /admin/v1/common/ws":     {httpStatus: http.StatusUnauthorized},
	}

	for _, route := range sortedAdminRoutes() {
		if route.Auth != routers.AuthModeNone {
			continue
		}
		key := route.Method + " " + route.Path
		scenario, ok := scenarios[key]
		if !ok {
			t.Fatalf("不走统一登录中间件的接口缺少显式测试场景: %s", key)
		}
		t.Run(key, func(t *testing.T) {
			resp := performRequest(route.Method, concreteRoutePath(route.Path), scenario.body, "")
			if resp.Code != scenario.httpStatus {
				t.Fatalf("不走统一登录中间件的接口状态不符合场景: want=%d got=%d body=%s", scenario.httpStatus, resp.Code, resp.Body.String())
			}
		})
	}
}

// TestAllProtectedGetRoutesReachBusinessHandlersWithRealMySQL 确认所有受保护 GET 接口均能通过真实认证进入参数或业务层。
func TestAllProtectedGetRoutesReachBusinessHandlersWithRealMySQL(t *testing.T) {
	requireMySQL(t)
	for _, route := range sortedAdminRoutes() {
		if route.Method != http.MethodGet || route.Auth == routers.AuthModeNone {
			continue
		}
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			resp := performRequest(route.Method, concreteRoutePath(route.Path), nil, authorization)
			switch resp.Code {
			case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
				http.StatusInternalServerError, http.StatusServiceUnavailable:
				t.Fatalf("接口未正常进入参数或业务处理: http=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}
}

// TestAllProtectedMutationRoutesReachHandlersWithRealMySQL 让全部受保护写接口越过鉴权并进入表单或业务处理。
// 默认空 JSON 应停在参数校验；无必填参数或可能产生全局副作用的接口使用显式安全载荷。
func TestAllProtectedMutationRoutesReachHandlersWithRealMySQL(t *testing.T) {
	requireMySQL(t)
	overrides := map[string]string{
		"/admin/v1/admin-user/update-profile":    `{"email":"invalid-email"}`,
		"/admin/v1/log/request/export":           `{"limit":-1}`,
		"/admin/v1/log/request/mask-config":      `{"common":"invalid"}`,
		"/admin/v1/system/file/export":           `{"storage_driver":"invalid"}`,
		"/admin/v1/system/notification/read-all": `{"category":"test-auto-route-coverage-none"}`,
	}

	for _, route := range sortedAdminRoutes() {
		if route.Method == http.MethodGet || route.Method == http.MethodHead || route.Auth == routers.AuthModeNone {
			continue
		}
		if route.Path == "/admin/v1/common/ws-ticket" {
			// WebSocket 专项流程会消费自己创建的短期 ticket，避免在 Redis 留下测试数据。
			continue
		}
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			body := `{}`
			if override, ok := overrides[route.Path]; ok {
				body = override
			}
			resp := performRequest(route.Method, concreteRoutePath(route.Path), &body, authorization)
			switch resp.Code {
			case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
				http.StatusInternalServerError, http.StatusServiceUnavailable:
				t.Fatalf("写接口未正常进入参数或业务处理: http=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}
}

func sortedAdminRoutes() []*routers.RouteMeta {
	metaMap := routers.CollectRouteMeta(routers.AppRouteTree())
	routes := make([]*routers.RouteMeta, 0, len(metaMap))
	for _, route := range metaMap {
		if strings.HasPrefix(route.Path, "/admin/v1/") {
			routes = append(routes, route)
		}
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes
}

func concreteRoutePath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "*") {
			segments[i] = "route-coverage-value"
		}
	}
	return strings.Join(segments, "/")
}

func requestBodyForMethod(method string) *string {
	if method == http.MethodGet || method == http.MethodHead {
		return nil
	}
	body := `{}`
	return &body
}
