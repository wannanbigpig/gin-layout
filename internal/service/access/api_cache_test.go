package access

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
)

func TestApiRouteCacheServiceDefaultsWithoutDatabase(t *testing.T) {
	service := NewApiRouteCacheService()
	service.ResetMetrics()

	if got := service.GetApiName("/missing", "GET"); got != "" {
		t.Fatalf("expected empty api name, got %q", got)
	}

	if got := service.CheckoutRouteIsAuth("/missing", "GET"); !got {
		t.Fatal("expected route to default to auth-required when lookup fails")
	}
}

func TestApiRouteCacheServiceCacheKey(t *testing.T) {
	service := NewApiRouteCacheService()
	service.ResetMetrics()
	if got := service.cacheKey("/admin/v1/users", "GET"); got != "GET:/admin/v1/users" {
		t.Fatalf("unexpected cache key: %s", got)
	}
}

func TestApiRouteCacheServiceGetRouteInfoSingleflightDeduplicates(t *testing.T) {
	service := NewApiRouteCacheService()
	service.ResetMetrics()
	service.configProvider = func() *config.Conf {
		return &config.Conf{
			Redis: autoload.RedisConfig{Enable: false},
		}
	}
	var loadCalls int32
	service.loadRouteInfo = func(route string, method string) (*ApiRouteInfo, error) {
		atomic.AddInt32(&loadCalls, 1)
		time.Sleep(30 * time.Millisecond)
		return &ApiRouteInfo{IsAuth: 1, Name: "demo"}, nil
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, 16)

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			info, err := service.GetRouteInfo("/admin/v1/demo", "GET")
			if err != nil {
				errCh <- err
				return
			}
			if info == nil || info.Name != "demo" || info.IsAuth != 1 {
				errCh <- fmt.Errorf("unexpected route info: %#v", info)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if got := atomic.LoadInt32(&loadCalls); got != 1 {
		t.Fatalf("expected loadRouteInfo to be called once, got %d", got)
	}

	snapshot := service.MetricsSnapshot()
	if snapshot.RequestTotal != 16 {
		t.Fatalf("expected request_total=16, got %d", snapshot.RequestTotal)
	}
	if snapshot.CacheMissTotal != 16 {
		t.Fatalf("expected cache_miss_total=16, got %d", snapshot.CacheMissTotal)
	}
	if snapshot.SourceLoadTotal != 1 {
		t.Fatalf("expected source_load_total=1, got %d", snapshot.SourceLoadTotal)
	}
	if snapshot.SingleflightShared == 0 {
		t.Fatal("expected singleflight_shared_total > 0")
	}
	if snapshot.CacheHitTotal != 0 {
		t.Fatalf("expected cache_hit_total=0, got %d", snapshot.CacheHitTotal)
	}
	if snapshot.HitRate != 0 {
		t.Fatalf("expected hit_rate=0, got %f", snapshot.HitRate)
	}
}

func TestApiRouteCacheServiceResetMetrics(t *testing.T) {
	service := NewApiRouteCacheService()
	service.ResetMetrics()

	service.metrics.requestTotal.Store(3)
	service.metrics.cacheHitTotal.Store(2)
	service.metrics.cacheMissTotal.Store(1)
	service.metrics.sourceLoadTotal.Store(1)
	service.metrics.singleflightShared.Store(1)
	service.metrics.refreshBatchTotal.Store(2)
	service.metrics.refreshWriteTotal.Store(9)

	service.ResetMetrics()
	snapshot := service.MetricsSnapshot()

	if snapshot.RequestTotal != 0 ||
		snapshot.CacheHitTotal != 0 ||
		snapshot.CacheMissTotal != 0 ||
		snapshot.SourceLoadTotal != 0 ||
		snapshot.SingleflightShared != 0 ||
		snapshot.RefreshBatchTotal != 0 ||
		snapshot.RefreshWriteTotal != 0 ||
		snapshot.HitRate != 0 {
		t.Fatalf("expected metrics reset to zero, got %#v", snapshot)
	}
}

func TestApiRouteCacheServiceCheckoutRouteIsAuthUsesThreeStateMode(t *testing.T) {
	service := NewApiRouteCacheService()
	service.configProvider = func() *config.Conf {
		return &config.Conf{
			Redis: autoload.RedisConfig{Enable: false},
		}
	}

	service.loadRouteInfo = func(route string, method string) (*ApiRouteInfo, error) {
		switch route {
		case "/public":
			return &ApiRouteInfo{IsAuth: 0, Name: "public"}, nil
		case "/login-only":
			return &ApiRouteInfo{IsAuth: 1, Name: "login"}, nil
		case "/authz":
			return &ApiRouteInfo{IsAuth: 2, Name: "authz"}, nil
		default:
			return nil, fmt.Errorf("unexpected route: %s", route)
		}
	}

	if service.CheckoutRouteIsAuth("/public", "GET") {
		t.Fatal("expected public route to not require api permission")
	}
	if service.CheckoutRouteIsAuth("/login-only", "GET") {
		t.Fatal("expected login-only route to not require api permission")
	}
	if !service.CheckoutRouteIsAuth("/authz", "GET") {
		t.Fatal("expected authz route to require api permission")
	}
}

func TestRedisContextWithTimeoutHonorsParentDeadline(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelParent()

	ctx, cancel := redisContextWithTimeout(parent, time.Second)
	defer cancel()

	parentDeadline, ok := parent.Deadline()
	if !ok {
		t.Fatal("expected parent deadline")
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected derived deadline")
	}
	if deadline.After(parentDeadline) {
		t.Fatalf("expected derived deadline %v to not exceed parent deadline %v", deadline, parentDeadline)
	}
}

func TestApiRouteCacheServiceRefreshTempKeyUsesShadowKey(t *testing.T) {
	service := NewApiRouteCacheService()

	tempKey := service.refreshTempKey()
	if tempKey == apiRedisKey {
		t.Fatal("expected temp key to differ from live cache key")
	}
	expectedPrefix := apiRedisKey + ":refresh:"
	if len(tempKey) <= len(expectedPrefix) || tempKey[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("expected temp key prefix %q, got %q", expectedPrefix, tempKey)
	}
}
