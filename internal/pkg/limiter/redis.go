package limiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
)

// redisTokenBucketLuaScript 原子令牌桶脚本
// KEYS[1]: 限流 key (如 rate_limit:ip:127.0.0.1)
// ARGV[1]: burst (桶容量)
// ARGV[2]: rps (每秒生成令牌速率)
// ARGV[3]: now (当前秒级时间戳)
// ARGV[4]: requested (请求令牌数，固定 1)
const redisTokenBucketLuaScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local info = redis.call("HMGET", key, "tokens", "last_update")
local tokens = tonumber(info[1])
local last_update = tonumber(info[2])

if not tokens then
    tokens = capacity
    last_update = now
else
    local delta = math.max(0, now - last_update)
    tokens = math.min(capacity, tokens + delta * rate)
    last_update = now
end

if tokens >= requested then
    tokens = tokens - requested
    redis.call("HMSET", key, "tokens", tokens, "last_update", last_update)
    redis.call("EXPIRE", key, math.ceil(capacity / rate) * 2 + 10)
    return 1
else
    return 0
end
`

// RedisDriver 基于 Redis + Lua 的分布式令牌桶驱动，内嵌 MemoryDriver 作为高可用熔断降级。
type RedisDriver struct {
	client   redis.Cmdable
	script   *redis.Script
	fallback *MemoryDriver
	timeout  time.Duration
}

// NewRedisDriver 创建 Redis 分布式限流驱动。
func NewRedisDriver(client redis.Cmdable) *RedisDriver {
	return &RedisDriver{
		client:   client,
		script:   redis.NewScript(redisTokenBucketLuaScript),
		fallback: NewMemoryDriver(),
		timeout:  15 * time.Millisecond,
	}
}

// Close 释放资源。
func (d *RedisDriver) Close() {
	if d.fallback != nil {
		d.fallback.Close()
	}
}

// Allow 判定是否允许请求通过。Redis 异常或超时自动降级为本地内存令牌桶。
func (d *RedisDriver) Allow(ctx context.Context, key string, rps float64, burst int) (bool, error) {
	if d.client == nil {
		return d.fallback.Allow(ctx, key, rps, burst)
	}

	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = 1
	}

	redisKey := "rate_limit:" + key
	now := float64(time.Now().UnixNano()) / 1e9

	callCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	result, err := d.script.Run(callCtx, d.client, []string{redisKey}, burst, rps, now, 1).Result()
	if err != nil {
		log.Logger.Warn("redis rate limiter failed, fallback to memory", zap.Error(err), zap.String("key", key))
		return d.fallback.Allow(ctx, key, rps, burst)
	}

	resNum, ok := result.(int64)
	if !ok {
		return d.fallback.Allow(ctx, key, rps, burst)
	}

	return resNum == 1, nil
}
