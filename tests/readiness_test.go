package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/routers"
)

func TestAllRootRoutesHaveExplicitInterfaceTests(t *testing.T) {
	expected := map[string]bool{
		http.MethodGet + " /ping":             true,
		http.MethodGet + " /health/readiness": true,
	}
	for _, route := range routers.CollectRouteMeta(routers.AppRouteTree()) {
		if strings.HasPrefix(route.Path, "/admin/") {
			continue
		}
		key := route.Method + " " + route.Path
		if !expected[key] {
			t.Fatalf("根路由缺少显式接口测试: %s", key)
		}
		delete(expected, key)
	}
	if len(expected) != 0 {
		t.Fatalf("显式根路由场景与路由树不一致: %#v", expected)
	}
}

func TestReadinessRouteReportsConfiguredMySQLReady(t *testing.T) {
	router, err := SetupRouter()
	if err != nil {
		t.Fatalf("setup router failed: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/health/readiness", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK && recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Dependencies struct {
			MySQL struct {
				Enabled  bool   `json:"enabled"`
				Required bool   `json:"required"`
				Ready    bool   `json:"ready"`
				Message  string `json:"message"`
			} `json:"mysql"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode readiness response failed: %v, body=%s", err, recorder.Body.String())
	}
	if !payload.Dependencies.MySQL.Enabled || !payload.Dependencies.MySQL.Required || !payload.Dependencies.MySQL.Ready {
		t.Fatalf("configured MySQL is not ready: %#v", payload.Dependencies.MySQL)
	}
}
