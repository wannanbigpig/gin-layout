package data

import (
	"sync"
	"time"
)

const defaultRuntimeHealthTTL = 3 * time.Second

// RuntimeHealthStatus 表示依赖最近一次运行时探测结果。
type RuntimeHealthStatus struct {
	Ready     bool
	Error     error
	CheckedAt time.Time
}

type runtimeHealthCache struct {
	ttl    time.Duration
	mu     sync.Mutex
	status RuntimeHealthStatus
}

func newRuntimeHealthCache(ttl time.Duration) *runtimeHealthCache {
	if ttl <= 0 {
		ttl = defaultRuntimeHealthTTL
	}
	return &runtimeHealthCache{ttl: ttl}
}

func (c *runtimeHealthCache) Check(probe func() error) RuntimeHealthStatus {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.status.CheckedAt.IsZero() && now.Sub(c.status.CheckedAt) < c.ttl {
		return c.status
	}

	err := probe()
	c.status = RuntimeHealthStatus{
		Ready:     err == nil,
		Error:     err,
		CheckedAt: now,
	}
	return c.status
}

func (c *runtimeHealthCache) SeedReady() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = RuntimeHealthStatus{
		Ready:     true,
		CheckedAt: time.Now(),
	}
}

func (c *runtimeHealthCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = RuntimeHealthStatus{}
}
