package limiter

import (
	"context"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

// Manager 管理运行时的 Limiter Driver，支持配置热更时原子切换。
type Manager struct {
	mu           sync.RWMutex
	driverType   string
	activeDriver Driver
	memoryDriver *MemoryDriver
	redisClient  func() redis.Cmdable
}

var (
	defaultManager     *Manager
	defaultManagerOnce sync.Once
)

// DefaultManager 获取全局限流管理器单例。
func DefaultManager() *Manager {
	defaultManagerOnce.Do(func() {
		mem := NewMemoryDriver()
		defaultManager = &Manager{
			driverType:   DriverMemory,
			activeDriver: mem,
			memoryDriver: mem,
		}
	})
	return defaultManager
}

// SetRedisClientProvider 注入 Redis 客户端获取函数。
func (m *Manager) SetRedisClientProvider(provider func() redis.Cmdable) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.redisClient = provider
	if m.driverType == DriverRedis {
		var client redis.Cmdable
		if provider != nil {
			client = provider()
		}
		m.activeDriver = NewRedisDriver(client)
	}
}

// SwitchDriver 动态切换限流驱动（如从 memory 切换到 redis）。
func (m *Manager) SwitchDriver(driverType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	driverType = strings.ToLower(strings.TrimSpace(driverType))
	if driverType == m.driverType && m.activeDriver != nil {
		return
	}

	if driverType == DriverRedis {
		var client redis.Cmdable
		if m.redisClient != nil {
			client = m.redisClient()
		}
		m.activeDriver = NewRedisDriver(client)
		m.driverType = DriverRedis
		return
	}

	// 默认回退为 memory
	if m.memoryDriver == nil {
		m.memoryDriver = NewMemoryDriver()
	}
	m.activeDriver = m.memoryDriver
	m.driverType = DriverMemory
}

// Allow 代理调用当前生效的 Driver。
func (m *Manager) Allow(ctx context.Context, key string, rps float64, burst int) (bool, error) {
	m.mu.RLock()
	driver := m.activeDriver
	m.mu.RUnlock()

	if driver == nil {
		return true, nil
	}
	return driver.Allow(ctx, key, rps, burst)
}

// CurrentDriverType 返回当前驱动标识。
func (m *Manager) CurrentDriverType() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.driverType
}
