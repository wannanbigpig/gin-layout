package auth

import (
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

const (
	tokenTypeBearer   = "Bearer"
	blacklistPrefix   = "blacklist:"
	refreshLockPrefix = "refresh_token_lock:"
	refreshLockTTL    = 5 * time.Second // 锁的过期时间，防止死锁
	defaultTokenTTL   = 24 * time.Hour  // 默认 token 过期时间（当 token_expires 为 NULL 时使用）
)

// refreshTokenLock 内存锁存储（当Redis未启用时使用）。
type refreshTokenLock struct {
	mu    sync.RWMutex
	ttl   time.Duration
	locks *gocache.Cache
}

func newRefreshTokenLock(ttl, cleanupInterval time.Duration) *refreshTokenLock {
	return &refreshTokenLock{
		ttl:   ttl,
		locks: gocache.New(ttl, cleanupInterval),
	}
}

// getLock 获取或创建指定 key 的锁。
func (r *refreshTokenLock) getLock(key string) *sync.Mutex {
	r.mu.RLock()
	if lock, ok := r.locks.Get(key); ok {
		r.mu.RUnlock()
		return lock.(*sync.Mutex)
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if lock, ok := r.locks.Get(key); ok {
		return lock.(*sync.Mutex)
	}

	newLock := &sync.Mutex{}
	// 让缓存接管过期回收，避免为每个 key 启一个 sleep goroutine 模拟 TTL。
	r.locks.Set(key, newLock, r.ttl)

	return newLock
}

var (
	defaultRefreshLockStoreOnce sync.Once
	defaultRefreshLockStoreInst *refreshTokenLock
)

func defaultRefreshLockStore() *refreshTokenLock {
	defaultRefreshLockStoreOnce.Do(func() {
		defaultRefreshLockStoreInst = newRefreshTokenLock(refreshLockTTL, refreshLockTTL)
	})
	return defaultRefreshLockStoreInst
}

// TokenResponse Token响应体。
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresAt    int64  `json:"expires_at"`
}

// LoginLogInfo 登录日志信息。
type LoginLogInfo struct {
	IP            string `json:"ip"`
	UserAgent     string `json:"user_agent"`
	OS            string `json:"os"`
	Browser       string `json:"browser"`
	ExecutionTime int    `json:"execution_time"`
}
