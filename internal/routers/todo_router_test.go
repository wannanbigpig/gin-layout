package routers

import (
	"net/http"
	"testing"

	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

func TestPersonalTodoRoutesRequireLoginWithoutPermission(t *testing.T) {
	catalog, err := CompileRouteTree(AppRouteTreeWithOptions(BuildOptions{}))
	if err != nil {
		t.Fatalf("compile route catalog: %v", err)
	}
	for _, item := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin/v1/todo/list"},
		{http.MethodGet, "/admin/v1/todo/summary"},
		{http.MethodGet, "/admin/v1/todo/reminder-preference"},
		{http.MethodPost, "/admin/v1/todo/create"},
		{http.MethodPost, "/admin/v1/todo/update"},
		{http.MethodPost, "/admin/v1/todo/status"},
		{http.MethodPost, "/admin/v1/todo/delete"},
		{http.MethodPost, "/admin/v1/todo/reminder-preference"},
	} {
		meta := catalog.MetaMap()[utils.MD5(item.method+"_"+item.path)]
		if meta == nil {
			t.Fatalf("missing route %s %s", item.method, item.path)
		}
		if meta.Security != SecurityAccessToken || meta.Auth != AuthModeLogin {
			t.Fatalf("route %s %s security/auth=%s/%d", item.method, item.path, meta.Security, meta.Auth)
		}
	}
}
