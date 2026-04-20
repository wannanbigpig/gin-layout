package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/gin-gonic/gin"

	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	"github.com/wannanbigpig/gin-layout/internal/global"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

type stubRouteChecker struct {
	requiresAuth bool
}

func (s stubRouteChecker) CheckoutRouteIsAuth(string, string) bool {
	return s.requiresAuth
}

func TestAdminAuthHandlerWithDepsAllowsPublicRouteWhenDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enforcer := buildTestEnforcer(t)
	deps := permissionDeps{
		loadEnforcer: func() (*casbinx.CasbinEnforcer, error) {
			return &casbinx.CasbinEnforcer{Enforcer: enforcer}, nil
		},
		routeChecker: stubRouteChecker{requiresAuth: false},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.StoreAuthPrincipal(c, &auth.AuthPrincipal{
			UserID:       2,
			IsSuperAdmin: global.No,
		})
		c.Next()
	})
	router.Use(AdminAuthHandlerWithDeps(deps))
	router.GET("/public", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if recorder.Body.String() != "ok" {
		t.Fatalf("expected body ok, got %q", recorder.Body.String())
	}
}

func TestAdminAuthHandlerWithDepsReturnsServerErrWhenEnforcerLoadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deps := permissionDeps{
		loadEnforcer: func() (*casbinx.CasbinEnforcer, error) {
			return nil, errors.New("casbin unavailable")
		},
		routeChecker: stubRouteChecker{requiresAuth: true},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.StoreAuthPrincipal(c, &auth.AuthPrincipal{
			UserID:       2,
			IsSuperAdmin: global.No,
		})
		c.Next()
	})
	router.Use(AdminAuthHandlerWithDeps(deps))
	router.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	result := decodeResult(t, recorder.Body.Bytes())
	if result.Code != e.ServerErr {
		t.Fatalf("expected code %d, got %d", e.ServerErr, result.Code)
	}
}

func buildTestEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()

	m, err := casbinmodel.NewModelFromString(`
[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`)
	if err != nil {
		t.Fatalf("build casbin model failed: %v", err)
	}

	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatalf("build casbin enforcer failed: %v", err)
	}
	return enforcer
}

func decodeResult(t *testing.T, body []byte) *response.Result {
	t.Helper()

	result := new(response.Result)
	if err := json.Unmarshal(body, result); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, string(body))
	}
	return result
}
