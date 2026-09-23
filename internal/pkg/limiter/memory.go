package limiter

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// MemoryDriver 基于内存令牌桶实现的单实例限流驱动。
type MemoryDriver struct {
	mu       sync.Mutex
	visitors map[string]*ipLimiter
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewMemoryDriver 创建并启动内存限流驱动。
func NewMemoryDriver() *MemoryDriver {
	ctx, cancel := context.WithCancel(context.Background())
	return NewMemoryDriverWithContext(ctx, cancel)
}

// NewMemoryDriverWithContext 使用外部 context 创建并启动内存限流驱动。
func NewMemoryDriverWithContext(ctx context.Context, cancel context.CancelFunc) *MemoryDriver {
	d := &MemoryDriver{
		visitors: make(map[string]*ipLimiter),
		ctx:      ctx,
		cancel:   cancel,
	}
	go d.cleanupLoop()
	return d
}

func (d *MemoryDriver) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.mu.Lock()
			now := time.Now()
			for k, v := range d.visitors {
				if now.Sub(v.lastSeen) > 3*time.Minute {
					delete(d.visitors, k)
				}
			}
			d.mu.Unlock()
		}
	}
}

// Close 停止后台清理。
func (d *MemoryDriver) Close() {
	if d.cancel != nil {
		d.cancel()
	}
}

// Allow 判定是否允许请求通过。
func (d *MemoryDriver) Allow(_ context.Context, key string, rps float64, burst int) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = 1
	}

	v, exists := d.visitors[key]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(rps), burst)
		d.visitors[key] = &ipLimiter{limiter: limiter, lastSeen: time.Now()}
		return limiter.Allow(), nil
	}

	v.lastSeen = time.Now()
	if v.limiter.Limit() != rate.Limit(rps) {
		v.limiter.SetLimit(rate.Limit(rps))
	}
	if v.limiter.Burst() != burst {
		v.limiter.SetBurst(burst)
	}

	return v.limiter.Allow(), nil
}
