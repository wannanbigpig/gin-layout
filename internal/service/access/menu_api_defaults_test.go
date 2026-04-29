package access

import (
	"net/http"
	"testing"
)

func TestNewMenuAPIDefaultsServiceUsesIsolatedDefaultBindings(t *testing.T) {
	first := NewMenuAPIDefaultsService()
	second := NewMenuAPIDefaultsService()
	if len(first.bindings) == 0 || len(second.bindings) == 0 {
		t.Fatal("expected default bindings to be initialized")
	}

	originalRoute := second.bindings[0].Route
	first.bindings[0].Route = "/mutated-by-test"

	if second.bindings[0].Route != originalRoute {
		t.Fatal("expected default bindings to be isolated per service instance")
	}
}

func TestNewMenuAPIDefaultsServiceWithDepsClonesBindings(t *testing.T) {
	customBindings := []defaultMenuAPIBinding{
		{MenuCode: "menu:test", Route: "/custom", Method: "GET"},
	}

	service := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		Bindings: customBindings,
	})
	customBindings[0].Route = "/changed"

	if service.bindings[0].Route != "/custom" {
		t.Fatal("expected custom bindings to be cloned")
	}
}

func TestNewMenuAPIDefaultsServiceWithDepsAllowsEmptyBindings(t *testing.T) {
	service := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		Bindings: []defaultMenuAPIBinding{},
	})

	if len(service.bindings) != 0 {
		t.Fatalf("expected empty bindings, got %d", len(service.bindings))
	}
}

func TestDefaultMenuAPIBindingsCoverManagementRoutes(t *testing.T) {
	service := NewMenuAPIDefaultsService()
	bindingSet := make(map[string]struct{}, len(service.bindings))
	for _, binding := range service.bindings {
		bindingSet[binding.Method+" "+binding.Route] = struct{}{}
	}

	required := []string{
		http.MethodGet + " /admin/v1/system/config/list",
		http.MethodGet + " /admin/v1/system/config/detail",
		http.MethodPost + " /admin/v1/system/config/create",
		http.MethodPost + " /admin/v1/system/config/update",
		http.MethodGet + " /admin/v1/system/dict/type/list",
		http.MethodGet + " /admin/v1/system/dict/options",
		http.MethodGet + " /admin/v1/task/list",
		http.MethodPost + " /admin/v1/task/trigger",
		http.MethodPost + " /admin/v1/task/run/retry",
		http.MethodPost + " /admin/v1/task/run/cancel",
		http.MethodGet + " /admin/v1/log/request/list",
		http.MethodGet + " /admin/v1/log/request/detail",
		http.MethodPost + " /admin/v1/log/request/mask-config",
	}
	for _, route := range required {
		if _, ok := bindingSet[route]; !ok {
			t.Fatalf("missing default menu API binding for %s", route)
		}
	}
}
