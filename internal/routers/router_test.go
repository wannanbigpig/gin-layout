package routers

import (
	"net/http"
	"testing"

	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

func TestSetRoutersRegistersApiMetadata(t *testing.T) {
	routes := CollectAdminRouteMeta()

	menuCode := utils.MD5(http.MethodPost + "_/admin/v1/menu/update")
	menuRoute, ok := routes[menuCode]
	if !ok {
		t.Fatalf("missing route metadata for menu update")
	}
	if menuRoute.GroupCode != "menu" {
		t.Fatalf("unexpected group code: %s", menuRoute.GroupCode)
	}
	if menuRoute.Auth != 1 {
		t.Fatalf("unexpected auth flag: %d", menuRoute.Auth)
	}
}

func TestSetRoutersRegistersPermissionCriticalRoutes(t *testing.T) {
	routes := CollectAdminRouteMeta()

	checkTokenCode := utils.MD5(http.MethodGet + "_/admin/v1/auth/check-token")
	if route, ok := routes[checkTokenCode]; !ok || route.Auth != 0 {
		t.Fatalf("missing or invalid auth check-token route: %#v", route)
	}

	updateProfileCode := utils.MD5(http.MethodPost + "_/admin/v1/admin-user/update-profile")
	if route, ok := routes[updateProfileCode]; !ok || route.Auth != 0 {
		t.Fatalf("missing or invalid update-profile route: %#v", route)
	}

	fileCode := utils.MD5(http.MethodGet + "_/admin/v1/file/:uuid")
	if route, ok := routes[fileCode]; !ok || route.Auth != 0 {
		t.Fatalf("missing or invalid file route: %#v", route)
	}
}

func TestSetRoutersRegistersCriticalRoutes(t *testing.T) {
	engine := SetRouters()
	routeMap := make(map[string]bool)
	for _, route := range engine.Routes() {
		routeMap[route.Method+" "+route.Path] = true
	}

	required := []string{
		http.MethodGet + " /ping",
		http.MethodGet + " /admin/v1/admin-user/list",
		http.MethodPost + " /admin/v1/permission/update",
		http.MethodGet + " /admin/v1/menu/list",
		http.MethodGet + " /admin/v1/role/list",
		http.MethodGet + " /admin/v1/department/list",
		http.MethodGet + " /admin/v1/log/request/list",
		http.MethodGet + " /admin/v1/log/login/list",
	}

	for _, route := range required {
		if !routeMap[route] {
			t.Fatalf("missing registered route: %s", route)
		}
	}
}
