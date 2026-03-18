package config

import "testing"

func resetConfigReloadHandlersForTest(t *testing.T) {
	t.Helper()
	reloadHandlersMu.Lock()
	defer reloadHandlersMu.Unlock()
	reloadHandlers = nil
}

func TestRegisterConfigReloadHandlerDeduplicatesByName(t *testing.T) {
	resetConfigReloadHandlersForTest(t)

	RegisterConfigReloadHandler(ConfigReloadHandler{Name: "data", Priority: 20})
	RegisterConfigReloadHandler(ConfigReloadHandler{Name: "data", Priority: 10})

	reloadHandlersMu.RLock()
	defer reloadHandlersMu.RUnlock()

	if len(reloadHandlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(reloadHandlers))
	}
	if reloadHandlers[0].Priority != 10 {
		t.Fatalf("expected overwritten priority 10, got %d", reloadHandlers[0].Priority)
	}
}

func TestRegisterConfigReloadHandlerKeepsStablePriorityOrder(t *testing.T) {
	resetConfigReloadHandlersForTest(t)

	RegisterConfigReloadHandler(ConfigReloadHandler{Name: "warnings", Priority: 100})
	RegisterConfigReloadHandler(ConfigReloadHandler{Name: "logger", Priority: 10})
	RegisterConfigReloadHandler(ConfigReloadHandler{Name: "data", Priority: 20})

	reloadHandlersMu.RLock()
	defer reloadHandlersMu.RUnlock()

	if len(reloadHandlers) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(reloadHandlers))
	}

	got := []string{reloadHandlers[0].Name, reloadHandlers[1].Name, reloadHandlers[2].Name}
	want := []string{"logger", "data", "warnings"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected order: got %v want %v", got, want)
		}
	}
}

func TestRegisterConfigReloadHandlerIgnoresEmptyName(t *testing.T) {
	resetConfigReloadHandlersForTest(t)

	RegisterConfigReloadHandler(ConfigReloadHandler{Priority: 1})

	reloadHandlersMu.RLock()
	defer reloadHandlersMu.RUnlock()

	if len(reloadHandlers) != 0 {
		t.Fatalf("expected empty-name handler to be ignored, got %d handlers", len(reloadHandlers))
	}
}
