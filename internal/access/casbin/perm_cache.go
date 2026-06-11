package casbinx

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// permCacheEntry 描述一条权限判定缓存。
type permCacheEntry struct {
	// allowed 表示 Casbin Enforce 的结果。
	allowed bool
	// expiresAt 为缓存过期的 UnixNano 时间戳。
	expiresAt int64
}

// PermCache 是 Casbin Enforce 结果的本地缓存。
// key 形如 "userID:resource:action"，使用 sync.Map 保证读多写少场景下的并发性能。
// 缓存仅存储成功命中或拒绝的判定结果，不会缓存错误。
type PermCache struct {
	// entries 存放 key -> *permCacheEntry。
	entries sync.Map
	// ttl 缓存有效期，单位 ns，使用 atomic 支持并发更新。
	ttl atomic.Int64
}

const (
	// defaultPermCacheTTL 默认缓存有效期。
	defaultPermCacheTTL = 5 * time.Minute
)

// globalPermCache 是包级共享的权限缓存实例。
var globalPermCache = NewPermCache(defaultPermCacheTTL)

// NewPermCache 创建一个新的权限缓存实例。
// ttl 不大于 0 时回退到默认值（5 分钟）。
func NewPermCache(ttl time.Duration) *PermCache {
	pc := &PermCache{}
	if ttl <= 0 {
		ttl = defaultPermCacheTTL
	}
	pc.ttl.Store(int64(ttl))
	return pc
}

// GlobalPermCache 返回包级共享缓存实例。
func GlobalPermCache() *PermCache {
	return globalPermCache
}

// SetTTL 动态调整缓存有效期；非正数时忽略。
// 已存在的条目仍按写入时刻 + 旧 TTL 过期，新写入条目使用新 TTL。
func (pc *PermCache) SetTTL(ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	pc.ttl.Store(int64(ttl))
}

// TTL 返回当前缓存有效期。
func (pc *PermCache) TTL() time.Duration {
	return time.Duration(pc.ttl.Load())
}

// PermCacheKey 拼接缓存 key。
func PermCacheKey(userID uint, resource, action string) string {
	var b strings.Builder
	b.Grow(len(resource) + len(action) + 22)
	b.WriteString(strconv.FormatUint(uint64(userID), 10))
	b.WriteByte(':')
	b.WriteString(resource)
	b.WriteByte(':')
	b.WriteString(action)
	return b.String()
}

// Get 读取缓存。第二个返回值为 true 时表示命中且未过期。
func (pc *PermCache) Get(userID uint, resource, action string) (bool, bool) {
	key := PermCacheKey(userID, resource, action)
	v, ok := pc.entries.Load(key)
	if !ok {
		return false, false
	}
	entry, ok := v.(*permCacheEntry)
	if !ok || entry == nil {
		pc.entries.Delete(key)
		return false, false
	}
	if time.Now().UnixNano() > entry.expiresAt {
		pc.entries.Delete(key)
		return false, false
	}
	return entry.allowed, true
}

// Set 写入或刷新缓存条目，使用当前 TTL + 随机抖动（0~30s）防止缓存雪崩。
func (pc *PermCache) Set(userID uint, resource, action string, allowed bool) {
	jitter := time.Duration(rand.Intn(30)) * time.Second
	pc.entries.Store(PermCacheKey(userID, resource, action), &permCacheEntry{
		allowed:   allowed,
		expiresAt: time.Now().UnixNano() + pc.ttl.Load() + int64(jitter),
	})
}

// InvalidateUser 清除指定用户的所有权限缓存。
func (pc *PermCache) InvalidateUser(userID uint) {
	prefix := strconv.FormatUint(uint64(userID), 10) + ":"
	pc.entries.Range(func(k, _ any) bool {
		key, ok := k.(string)
		if !ok {
			return true
		}
		if strings.HasPrefix(key, prefix) {
			pc.entries.Delete(key)
		}
		return true
	})
}

// InvalidateAll 清空全部权限缓存。
func (pc *PermCache) InvalidateAll() {
	pc.entries.Range(func(k, _ any) bool {
		pc.entries.Delete(k)
		return true
	})
}

// InvalidateUserPermCache 是包级便捷函数，清除指定用户在共享缓存中的全部条目。
func InvalidateUserPermCache(userID uint) {
	globalPermCache.InvalidateUser(userID)
}

// InvalidateUsersPermCache 批量清除多个用户的缓存。
func InvalidateUsersPermCache(userIDs []uint) {
	if len(userIDs) == 0 {
		return
	}
	for _, id := range userIDs {
		globalPermCache.InvalidateUser(id)
	}
}

// InvalidateAllPermCache 是包级便捷函数，清空共享缓存中的全部条目。
func InvalidateAllPermCache() {
	globalPermCache.InvalidateAll()
}

// SetPermCacheTTL 调整共享缓存 TTL。
func SetPermCacheTTL(ttl time.Duration) {
	globalPermCache.SetTTL(ttl)
}
