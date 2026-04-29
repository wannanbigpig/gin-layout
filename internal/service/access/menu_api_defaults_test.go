package access

import (
	"net/http"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
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

func TestCollectMenuAPIBindingKeysKeepsUniqueValues(t *testing.T) {
	bindings := []defaultMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		{MenuCode: "menu:update", Route: "/admin/v1/menu/update", Method: http.MethodPost},
	}

	menuCodes, routes, methods := collectMenuAPIBindingKeys(bindings)

	if len(menuCodes) != 2 || menuCodes[0] != "menu:list" || menuCodes[1] != "menu:update" {
		t.Fatalf("unexpected menu codes: %#v", menuCodes)
	}
	if len(routes) != 2 || routes[0] != "/admin/v1/menu/list" || routes[1] != "/admin/v1/menu/update" {
		t.Fatalf("unexpected routes: %#v", routes)
	}
	if len(methods) != 2 || methods[0] != http.MethodGet || methods[1] != http.MethodPost {
		t.Fatalf("unexpected methods: %#v", methods)
	}
}

func TestBuildDefaultMenuAPIMappingsSkipsMissingTargets(t *testing.T) {
	bindings := []defaultMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		{MenuCode: "menu:missing", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		{MenuCode: "menu:list", Route: "/admin/v1/menu/missing", Method: http.MethodGet},
	}
	targets := defaultMenuAPITargets{
		menuIDByCode: map[string]uint{
			"menu:list": 10,
		},
		apiIDByRouteMethod: map[string]uint{
			http.MethodGet + ":/admin/v1/menu/list": 20,
		},
	}

	mappings := buildDefaultMenuAPIMappings(bindings, targets)

	if len(mappings) != 1 {
		t.Fatalf("expected one mapping, got %d", len(mappings))
	}
	want := &model.MenuApiMap{MenuId: 10, ApiId: 20}
	if mappings[0].MenuId != want.MenuId || mappings[0].ApiId != want.ApiId {
		t.Fatalf("unexpected mapping: got=%+v want=%+v", mappings[0], want)
	}
}
