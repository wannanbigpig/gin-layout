package limiter

import (
	"context"
	"testing"
)

func TestMemoryDriverAllow(t *testing.T) {
	d := NewMemoryDriver()
	defer d.Close()

	ctx := context.Background()
	key := "test-client-1"

	// burst 2, rps 1
	allowed, err := d.Allow(ctx, key, 1, 2)
	if err != nil || !allowed {
		t.Fatalf("first request should be allowed, got: %v, err: %v", allowed, err)
	}

	allowed, err = d.Allow(ctx, key, 1, 2)
	if err != nil || !allowed {
		t.Fatalf("second request should be allowed due to burst=2, got: %v, err: %v", allowed, err)
	}

	// 第 3 次应被限流
	allowed, err = d.Allow(ctx, key, 1, 2)
	if err != nil || allowed {
		t.Fatalf("third request should be rejected, got: %v", allowed)
	}
}

func TestRedisDriverFallbackWhenClientNil(t *testing.T) {
	d := NewRedisDriver(nil)
	defer d.Close()

	ctx := context.Background()
	key := "test-client-nil"

	allowed, err := d.Allow(ctx, key, 1, 1)
	if err != nil || !allowed {
		t.Fatalf("nil redis client should fallback to memory and succeed, got: %v, err: %v", allowed, err)
	}

	allowed, err = d.Allow(ctx, key, 1, 1)
	if err != nil || allowed {
		t.Fatalf("second request should be limited by memory fallback, got: %v", allowed)
	}
}

func TestManagerSwitchDriver(t *testing.T) {
	mgr := DefaultManager()
	if mgr.CurrentDriverType() != DriverMemory {
		t.Fatalf("default driver should be memory, got: %s", mgr.CurrentDriverType())
	}

	mgr.SwitchDriver(DriverRedis)
	if mgr.CurrentDriverType() != DriverRedis {
		t.Fatalf("driver should be switched to redis, got: %s", mgr.CurrentDriverType())
	}

	// 无 redisClient 情况下调用 Allow 应自动降级不报错
	allowed, err := mgr.Allow(context.Background(), "test-mgr", 10, 10)
	if err != nil || !allowed {
		t.Fatalf("allow should pass via fallback, got: %v, err: %v", allowed, err)
	}

	mgr.SwitchDriver(DriverMemory)
	if mgr.CurrentDriverType() != DriverMemory {
		t.Fatalf("driver should be switched back to memory, got: %s", mgr.CurrentDriverType())
	}
}
