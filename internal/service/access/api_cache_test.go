package access

import "testing"

func TestApiRouteCacheServiceDefaultsWithoutDatabase(t *testing.T) {
	service := NewApiRouteCacheService()

	if got := service.GetApiName("/missing", "GET"); got != "" {
		t.Fatalf("expected empty api name, got %q", got)
	}

	if got := service.CheckoutRouteIsAuth("/missing", "GET"); !got {
		t.Fatal("expected route to default to auth-required when lookup fails")
	}
}

func TestApiRouteCacheServiceCacheKey(t *testing.T) {
	service := NewApiRouteCacheService()
	if got := service.cacheKey("/admin/v1/users", "GET"); got != "GET:/admin/v1/users" {
		t.Fatalf("unexpected cache key: %s", got)
	}
}
