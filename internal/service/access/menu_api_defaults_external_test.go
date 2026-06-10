package access

import (
	"net/http"
	"testing"
)

func TestAddExternalBinding(t *testing.T) {
	ClearExternalBindings()
	defer ClearExternalBindings()

	AddExternalBinding(ExternalMenuAPIBinding{
		MenuCode: "product:list",
		Route:    "/admin/v1/product/list",
		Method:   http.MethodGet,
	})

	bindings := LoadExternalBindings()
	if len(bindings) != 1 {
		t.Fatalf("expected 1 external binding, got %d", len(bindings))
	}
	if bindings[0].MenuCode != "product:list" {
		t.Fatalf("unexpected menu code: %s", bindings[0].MenuCode)
	}
	if bindings[0].Source != BindingSourceExternal {
		t.Fatalf("expected source external, got %s", bindings[0].Source)
	}
}

func TestAddExternalBindings(t *testing.T) {
	ClearExternalBindings()
	defer ClearExternalBindings()

	bindings := []ExternalMenuAPIBinding{
		{MenuCode: "product:list", Route: "/admin/v1/product/list", Method: http.MethodGet},
		{MenuCode: "product:create", Route: "/admin/v1/product/create", Method: http.MethodPost},
	}
	AddExternalBindings(bindings)

	loaded := LoadExternalBindings()
	if len(loaded) != 2 {
		t.Fatalf("expected 2 external bindings, got %d", len(loaded))
	}
}

func TestLoadExternalBindingsReturnsClone(t *testing.T) {
	ClearExternalBindings()
	defer ClearExternalBindings()

	AddExternalBinding(ExternalMenuAPIBinding{
		MenuCode: "test:code",
		Route:    "/test",
		Method:   http.MethodGet,
	})

	loaded := LoadExternalBindings()
	loaded[0].MenuCode = "mutated"

	original := LoadExternalBindings()
	if original[0].MenuCode == "mutated" {
		t.Fatal("expected LoadExternalBindings to return a clone")
	}
}

func TestClearExternalBindings(t *testing.T) {
	ClearExternalBindings()
	defer ClearExternalBindings()

	AddExternalBinding(ExternalMenuAPIBinding{
		MenuCode: "test:code",
		Route:    "/test",
		Method:   http.MethodGet,
	})

	ClearExternalBindings()
	bindings := LoadExternalBindings()
	if len(bindings) != 0 {
		t.Fatalf("expected 0 external bindings after clear, got %d", len(bindings))
	}
}

func TestMergeBindingsNoDuplicates(t *testing.T) {
	builtin := []defaultMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		{MenuCode: "menu:update", Route: "/admin/v1/menu/update", Method: http.MethodPost},
	}
	external := []ExternalMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet}, // duplicate
		{MenuCode: "product:list", Route: "/admin/v1/product/list", Method: http.MethodGet},
	}

	merged := MergeBindings(builtin, external)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged bindings, got %d", len(merged))
	}
}

func TestMergeBindingsEmptyBuiltin(t *testing.T) {
	external := []ExternalMenuAPIBinding{
		{MenuCode: "product:list", Route: "/admin/v1/product/list", Method: http.MethodGet},
	}

	merged := MergeBindings(nil, external)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged binding, got %d", len(merged))
	}
	if merged[0].MenuCode != "product:list" {
		t.Fatalf("unexpected menu code: %s", merged[0].MenuCode)
	}
}

func TestMergeBindingsEmptyExternal(t *testing.T) {
	builtin := []defaultMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
	}

	merged := MergeBindings(builtin, nil)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged binding, got %d", len(merged))
	}
}

func TestGetBindingStats(t *testing.T) {
	builtin := []defaultMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
	}
	external := []ExternalMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet}, // duplicate
		{MenuCode: "product:list", Route: "/admin/v1/product/list", Method: http.MethodGet},
	}

	stats := GetBindingStats(builtin, external)
	if stats["builtin_count"] != 1 {
		t.Fatalf("expected builtin_count=1, got %d", stats["builtin_count"])
	}
	if stats["external_count"] != 2 {
		t.Fatalf("expected external_count=2, got %d", stats["external_count"])
	}
	if stats["total_count"] != 2 {
		t.Fatalf("expected total_count=2, got %d", stats["total_count"])
	}
	if stats["duplicate_count"] != 1 {
		t.Fatalf("expected duplicate_count=1, got %d", stats["duplicate_count"])
	}
}

func TestBindingDiff(t *testing.T) {
	builtin := []defaultMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
	}
	external := []ExternalMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet}, // exists in builtin
		{MenuCode: "product:list", Route: "/admin/v1/product/list", Method: http.MethodGet},
	}

	diff := BindingDiff(builtin, external)
	if len(diff) != 1 {
		t.Fatalf("expected 1 diff binding, got %d", len(diff))
	}
	if diff[0].MenuCode != "product:list" {
		t.Fatalf("unexpected menu code in diff: %s", diff[0].MenuCode)
	}
}

func TestNewMenuAPIDefaultsServiceWithExternalBindings(t *testing.T) {
	ClearExternalBindings()
	defer ClearExternalBindings()

	AddExternalBinding(ExternalMenuAPIBinding{
		MenuCode: "product:list",
		Route:    "/admin/v1/product/list",
		Method:   http.MethodGet,
	})

	service := NewMenuAPIDefaultsService()
	stats := service.BindingStats()

	if stats["external_count"] != 1 {
		t.Fatalf("expected 1 external binding, got %d", stats["external_count"])
	}
	if stats["builtin_count"] == 0 {
		t.Fatal("expected builtin bindings to be loaded")
	}
}

func TestNewMenuAPIDefaultsServiceWithDepsExternalBindings(t *testing.T) {
	service := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		ExternalBindings: []ExternalMenuAPIBinding{
			{MenuCode: "product:list", Route: "/admin/v1/product/list", Method: http.MethodGet},
		},
	})

	stats := service.BindingStats()
	if stats["external_count"] != 1 {
		t.Fatalf("expected 1 external binding, got %d", stats["external_count"])
	}
}

func TestNewMenuAPIDefaultsServiceWithoutExternalBindings(t *testing.T) {
	ClearExternalBindings()
	defer ClearExternalBindings()

	AddExternalBinding(ExternalMenuAPIBinding{
		MenuCode: "product:list",
		Route:    "/admin/v1/product/list",
		Method:   http.MethodGet,
	})

	includeExternal := false
	service := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		IncludeExternal: &includeExternal,
	})

	stats := service.BindingStats()
	if stats["external_count"] != 0 {
		t.Fatalf("expected 0 external bindings when disabled, got %d", stats["external_count"])
	}
}

func TestMenuAPIDefaultsServiceAddExternalBinding(t *testing.T) {
	service := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		Bindings: []defaultMenuAPIBinding{
			{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		},
	})

	service.AddExternalBinding(ExternalMenuAPIBinding{
		MenuCode: "product:list",
		Route:    "/admin/v1/product/list",
		Method:   http.MethodGet,
	})

	stats := service.BindingStats()
	if stats["external_count"] != 1 {
		t.Fatalf("expected 1 external binding after add, got %d", stats["external_count"])
	}
	if stats["total_count"] != 2 {
		t.Fatalf("expected 2 total bindings after add, got %d", stats["total_count"])
	}
}

func TestMenuAPIDefaultsServiceBuiltinBindings(t *testing.T) {
	service := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		Bindings: []defaultMenuAPIBinding{
			{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		},
	})

	builtin := service.BuiltinBindings()
	if len(builtin) != 1 {
		t.Fatalf("expected 1 builtin binding, got %d", len(builtin))
	}

	external := service.ExternalBindings()
	if len(external) != 0 {
		t.Fatalf("expected 0 external bindings, got %d", len(external))
	}
}

func TestDefaultMenuAPIBindingRouteSetIncludesExternal(t *testing.T) {
	ClearExternalBindings()
	defer ClearExternalBindings()

	AddExternalBinding(ExternalMenuAPIBinding{
		MenuCode: "generated:list",
		Route:    "/admin/v1/generated/list",
		Method:   http.MethodGet,
	})

	routeSet := DefaultMenuAPIBindingRouteSet()
	key := http.MethodGet + " /admin/v1/generated/list"
	if _, ok := routeSet[key]; !ok {
		t.Fatal("expected external binding route to be included in route set")
	}
}

func TestBuiltinDefaultMenuAPIBindingRouteSetExcludesExternal(t *testing.T) {
	ClearExternalBindings()
	defer ClearExternalBindings()

	AddExternalBinding(ExternalMenuAPIBinding{
		MenuCode: "generated:list",
		Route:    "/admin/v1/generated/list",
		Method:   http.MethodGet,
	})

	routeSet := BuiltinDefaultMenuAPIBindingRouteSet()
	key := http.MethodGet + " /admin/v1/generated/list"
	if _, ok := routeSet[key]; ok {
		t.Fatal("expected external binding route to be excluded from builtin route set")
	}
}

func TestFormatExternalBindings(t *testing.T) {
	bindings := []ExternalMenuAPIBinding{
		{MenuCode: "product:list", Route: "/admin/v1/product/list", Method: http.MethodGet, Source: BindingSourceExternal, Description: "产品列表"},
	}

	code := FormatExternalBindings(bindings)
	if code == "" {
		t.Fatal("expected non-empty code output")
	}
	if !containsString(code, "product:list") {
		t.Fatal("expected code to contain menu code")
	}
}

func TestFormatExternalBindingsEmpty(t *testing.T) {
	code := FormatExternalBindings(nil)
	if code != "" {
		t.Fatal("expected empty code output for nil bindings")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
