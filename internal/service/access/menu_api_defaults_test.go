package access

import "testing"

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
