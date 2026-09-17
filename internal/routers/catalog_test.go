package routers

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

func TestBuildOptionsControlDemoIndependentlyOfGinMode(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousMode) })

	gin.SetMode(gin.ReleaseMode)
	debugCatalog, err := CompileRouteTree(AppRouteTreeWithOptions(BuildOptions{Debug: true}))
	if err != nil {
		t.Fatalf("compile debug catalog: %v", err)
	}
	if _, ok := debugCatalog.MetaMap()[utils.MD5(http.MethodGet+"_/admin/v1/demo")]; !ok {
		t.Fatal("debug catalog should contain demo route independently of gin mode")
	}

	gin.SetMode(gin.DebugMode)
	releaseCatalog, err := CompileRouteTree(AppRouteTreeWithOptions(BuildOptions{Debug: false}))
	if err != nil {
		t.Fatalf("compile release catalog: %v", err)
	}
	if _, ok := releaseCatalog.MetaMap()[utils.MD5(http.MethodGet+"_/admin/v1/demo")]; ok {
		t.Fatal("release catalog should exclude demo route independently of gin mode")
	}
}

func TestCompileRouteTreeResolvesSecurityAndHandlerOrder(t *testing.T) {
	trace := make([]string, 0, 3)
	middlewareHandler := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) {
			trace = append(trace, name)
			c.Next()
		}
	}
	endpoint := func(c *gin.Context) {
		trace = append(trace, "endpoint")
		c.Status(http.StatusNoContent)
	}
	tree := RouteGroupDef{
		Prefix:     "root",
		GroupCode:  "root",
		Security:   SecurityPublic,
		Middleware: []gin.HandlerFunc{middlewareHandler("root")},
		Children: []RouteGroupDef{{
			Prefix:     "child",
			GroupCode:  "child",
			Security:   SecurityRefreshCookie,
			Middleware: []gin.HandlerFunc{middlewareHandler("child")},
			Routes:     []RouteDef{GET("item", "item", endpoint)},
		}},
	}
	catalog, err := CompileRouteTree(tree)
	if err != nil {
		t.Fatalf("compile catalog: %v", err)
	}
	routes := catalog.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected one route, got %d", len(routes))
	}
	got := routes[0]
	if got.Path != "/root/child/item" || got.GroupCode != "child" || got.Security != SecurityRefreshCookie || got.Auth != AuthModeNone {
		t.Fatalf("unexpected compiled route: %#v", got.RouteMeta)
	}
	if len(got.Handlers) != 3 {
		t.Fatalf("expected 3 effective handlers, got %d", len(got.Handlers))
	}

	engine := gin.New()
	RegisterCatalog(engine, catalog)
	request := httptest.NewRequest(http.MethodGet, got.Path, nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if !reflect.DeepEqual(trace, []string{"root", "child", "endpoint"}) {
		t.Fatalf("unexpected handler order: %#v", trace)
	}
}

func TestRegisterCatalogPublishesTrustedOperationNameBeforeRouteHandlers(t *testing.T) {
	var operationName string
	tree := RouteGroupDef{
		Prefix:   "root",
		Security: SecurityPublic,
		Routes: []RouteDef{GET("item", "可信操作名", func(c *gin.Context) {
			operationName = c.GetString(global.ContextKeyOperationName)
			c.Status(http.StatusNoContent)
		})},
	}
	catalog, err := CompileRouteTree(tree)
	if err != nil {
		t.Fatalf("compile catalog: %v", err)
	}
	engine := gin.New()
	RegisterCatalog(engine, catalog)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/root/item", nil)
	request.Header.Set("X-Operation-Name", "客户端伪造名称")
	engine.ServeHTTP(recorder, request)
	if operationName != "可信操作名" {
		t.Fatalf("operation name = %q, want catalog title", operationName)
	}
}

func TestApplicationCatalogSecurityProjection(t *testing.T) {
	catalog, err := CompileRouteTree(AppRouteTreeWithOptions(BuildOptions{}))
	if err != nil {
		t.Fatalf("compile application catalog: %v", err)
	}
	cases := []struct {
		method   string
		path     string
		security SecurityPolicy
		auth     AuthMode
	}{
		{http.MethodPost, "/admin/v1/login", SecurityPublic, AuthModeNone},
		{http.MethodPost, "/admin/v1/auth/refresh", SecurityRefreshCookie, AuthModeNone},
		{http.MethodGet, "/admin/v1/common/ws", SecurityWebSocketTicket, AuthModeNone},
		{http.MethodGet, "/admin/v1/auth/check-token", SecurityAccessToken, AuthModeLogin},
		{http.MethodGet, "/admin/v1/dashboard/overview", SecurityAccessTokenPermission, AuthModeAuth},
	}
	for _, tc := range cases {
		meta := catalog.MetaMap()[utils.MD5(tc.method+"_"+tc.path)]
		if meta == nil {
			t.Fatalf("missing %s %s", tc.method, tc.path)
		}
		if meta.Security != tc.security || meta.Auth != tc.auth {
			t.Errorf("%s %s security/auth = %s/%d, want %s/%d", tc.method, tc.path, meta.Security, meta.Auth, tc.security, tc.auth)
		}
	}
}

func TestDebugCatalogIsReleaseCatalogPlusDemo(t *testing.T) {
	debugCatalog, err := CompileRouteTree(AppRouteTreeWithOptions(BuildOptions{Debug: true}))
	if err != nil {
		t.Fatalf("compile debug catalog: %v", err)
	}
	releaseCatalog, err := CompileRouteTree(AppRouteTreeWithOptions(BuildOptions{Debug: false}))
	if err != nil {
		t.Fatalf("compile release catalog: %v", err)
	}
	releaseRoutes := make(map[string]struct{}, len(releaseCatalog.Routes()))
	for _, route := range releaseCatalog.Routes() {
		releaseRoutes[route.Method+" "+route.Path] = struct{}{}
	}
	var extra []string
	for _, route := range debugCatalog.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := releaseRoutes[key]; !ok {
			extra = append(extra, key)
		}
	}
	if !reflect.DeepEqual(extra, []string{"GET /admin/v1/demo"}) {
		t.Fatalf("unexpected debug-only routes: %#v", extra)
	}
}
