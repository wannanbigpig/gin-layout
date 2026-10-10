package access

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/redis/go-redis/v9"
)

func useDataScopeTestVersion(t *testing.T, initial uint64) *atomic.Uint64 {
	t.Helper()
	previousVersion, previousRedis := currentDataScopeCacheVersion, dataScopeRedisClient
	version := &atomic.Uint64{}
	version.Store(initial)
	currentDataScopeCacheVersion = func(context.Context) (uint64, error) { return version.Load(), nil }
	dataScopeRedisClient = func() *redis.Client { return nil }
	t.Cleanup(func() { currentDataScopeCacheVersion, dataScopeRedisClient = previousVersion, previousRedis })
	return version
}

// 模拟共享 Redis 的 GET/SET；不建立网络连接，两个过滤器读取同一份序列化缓存。
type dataScopeRedisHook struct {
	sync.Mutex
	values   map[string]string
	afterSet func()
}

func (h *dataScopeRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *dataScopeRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *dataScopeRedisHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		h.Lock()
		defer h.Unlock()
		key := fmt.Sprint(cmd.Args()[1])
		switch cmd.Name() {
		case "get":
			value, found := h.values[key]
			if !found {
				return redis.Nil
			}
			cmd.(*redis.StringCmd).SetVal(value)
		case "set":
			if value, ok := cmd.Args()[2].([]byte); ok {
				h.values[key] = string(value)
			} else {
				h.values[key] = fmt.Sprint(cmd.Args()[2])
			}
			cmd.(*redis.StatusCmd).SetVal("OK")
			if h.afterSet != nil {
				h.afterSet()
			}
		default:
			return fmt.Errorf("unexpected Redis command %s", cmd.Name())
		}
		return nil
	}
}

func useDataScopeSharedRedis(t *testing.T) *dataScopeRedisHook {
	t.Helper()
	hook := &dataScopeRedisHook{values: make(map[string]string)}
	client := redis.NewClient(&redis.Options{Addr: "unused", MaxRetries: -1})
	client.AddHook(hook)
	dataScopeRedisClient = func() *redis.Client { return client }
	t.Cleanup(func() { _ = client.Close() })
	return hook
}

func stubDataScopeCompute(t *testing.T, fn func(*DataScopeFilter, context.Context, uint) (*scopeResult, error)) {
	t.Helper()
	previous := computeScopeForSingleflight
	computeScopeForSingleflight = fn
	t.Cleanup(func() { computeScopeForSingleflight = previous })
}

func TestDataScopeFiltersReuseSharedVersionAndRejectRemoteRevocation(t *testing.T) {
	version := useDataScopeTestVersion(t, 1)
	useDataScopeSharedRedis(t)
	calls := 0
	stubDataScopeCompute(t, func(_ *DataScopeFilter, _ context.Context, uid uint) (*scopeResult, error) {
		calls++
		if version.Load() == 1 {
			return &scopeResult{ScopeAll: true}, nil
		}
		return &scopeResult{OwnerUID: uid, HasOwnerUID: true}, nil
	})
	first, second := NewDataScopeFilter(), NewDataScopeFilter()
	input := DataScopeInput{UID: 701}
	for _, filter := range []*DataScopeFilter{first, second, first, second} {
		cond, _ := filter.Build(context.Background(), input, "dept_id", "owner_id")
		if cond != "" {
			t.Fatalf("expected all scope before revocation, got %q", cond)
		}
	}
	if calls != 1 {
		t.Fatalf("shared cache should need one compute, got %d", calls)
	}
	// 另一进程提交事务，只推进共享版本；即使未收到删除通知也必须重新计算。
	version.Add(1)
	cond, _ := second.Build(context.Background(), input, "dept_id", "owner_id")
	if cond != "owner_id = ?" || calls != 2 {
		t.Fatalf("stale scope survived remote revocation: condition=%q computes=%d", cond, calls)
	}
}

func testDataScopeMutationDuringCompute(t *testing.T, invalidateAll bool) {
	version := useDataScopeTestVersion(t, 1)
	calls := 0
	stubDataScopeCompute(t, func(f *DataScopeFilter, ctx context.Context, uid uint) (*scopeResult, error) {
		calls++
		if calls == 1 {
			version.Add(1) // 模拟另一权限写事务提交，共享版本同步改变。
			if invalidateAll {
				f.InvalidateAllCache(ctx)
			} else {
				f.InvalidateCache(ctx, uid)
			}
			return &scopeResult{ScopeAll: true}, nil
		}
		return &scopeResult{OwnerUID: uid, HasOwnerUID: true}, nil
	})
	cond, _ := NewDataScopeFilter().Build(context.Background(), DataScopeInput{UID: 702}, "dept_id", "owner_id")
	if cond != "owner_id = ?" || calls != 2 {
		t.Fatalf("in-flight revoked result was returned: condition=%q computes=%d", cond, calls)
	}
}

func TestDataScopeRechecksVersionAfterLateCacheWrite(t *testing.T) {
	version := useDataScopeTestVersion(t, 1)
	hook := useDataScopeSharedRedis(t)
	hook.afterSet = func() { version.CompareAndSwap(1, 2) }
	calls := 0
	stubDataScopeCompute(t, func(_ *DataScopeFilter, _ context.Context, uid uint) (*scopeResult, error) {
		calls++
		if version.Load() == 1 {
			return &scopeResult{ScopeAll: true}, nil
		}
		return &scopeResult{OwnerUID: uid, HasOwnerUID: true}, nil
	})
	cond, _ := NewDataScopeFilter().Build(context.Background(), DataScopeInput{UID: 703}, "dept_id", "owner_id")
	if cond != "owner_id = ?" || calls != 2 {
		t.Fatalf("late cache write returned revoked scope: condition=%q computes=%d", cond, calls)
	}
}

func TestDataScopeFailsClosedWhenVersionRecheckFails(t *testing.T) {
	useDataScopeTestVersion(t, 1)
	stubDataScopeCompute(t, func(_ *DataScopeFilter, _ context.Context, _ uint) (*scopeResult, error) {
		currentDataScopeCacheVersion = func(context.Context) (uint64, error) { return 0, errors.New("version unavailable") }
		return &scopeResult{ScopeAll: true}, nil
	})
	cond, _ := NewDataScopeFilter().Build(context.Background(), DataScopeInput{UID: 704}, "dept_id", "owner_id")
	if cond != "1=0" {
		t.Fatalf("version failure must deny access, got %q", cond)
	}
}

func TestDataScopeRepeatedConcurrentRevocationFailsClosed(t *testing.T) {
	version := useDataScopeTestVersion(t, 1)
	stubDataScopeCompute(t, func(_ *DataScopeFilter, _ context.Context, _ uint) (*scopeResult, error) {
		version.Add(1)
		return &scopeResult{ScopeAll: true}, nil
	})
	cond, _ := NewDataScopeFilter().Build(context.Background(), DataScopeInput{UID: 705}, "dept_id", "owner_id")
	if cond != "1=0" {
		t.Fatalf("unstable version must deny access, got %q", cond)
	}
}
