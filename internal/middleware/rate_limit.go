package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/pkg/limiter"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Driver  string // memory 或 redis
	Enabled bool
	RPS     float64
	Burst   int
	Name    string
}

// RateLimitHandler 全局 IP 级速率限制中间件。
func RateLimitHandler(cfg RateLimitConfig) gin.HandlerFunc {
	return RateLimitHandlerWithContext(context.Background(), cfg)
}

// RateLimitHandlerWithContext 返回可随 ctx 结束而停止清理循环的限流中间件。
func RateLimitHandlerWithContext(ctx context.Context, cfg RateLimitConfig) gin.HandlerFunc {
	return DynamicRateLimitHandlerWithContext(ctx, func() RateLimitConfig {
		return cfg
	})
}

// DynamicRateLimitHandler 接收动态配置提供者函数，使限流器配置能够随热更新动态生效。
func DynamicRateLimitHandler(provider func() RateLimitConfig) gin.HandlerFunc {
	return DynamicRateLimitHandlerWithContext(context.Background(), provider)
}

// DynamicRateLimitHandlerWithContext 返回动态读取配置并随 ctx 结束而停止清理循环的限流中间件。
func DynamicRateLimitHandlerWithContext(ctx context.Context, provider func() RateLimitConfig) gin.HandlerFunc {
	if provider == nil {
		return func(c *gin.Context) { c.Next() }
	}
	if ctx == nil {
		ctx = context.Background()
	}

	subCtx, cancel := context.WithCancel(ctx)
	memDriver := limiter.NewMemoryDriverWithContext(subCtx, cancel)

	var (
		mu          sync.RWMutex
		lastDriver  string
		redisDriver *limiter.RedisDriver
	)

	return func(c *gin.Context) {
		cfg := provider()
		if !cfg.Enabled {
			c.Next()
			return
		}

		key := c.ClientIP()
		if cfg.Name != "" {
			key = cfg.Name + ":" + key
		}

		driverType := strings.ToLower(strings.TrimSpace(cfg.Driver))
		if driverType == limiter.DriverRedis {
			mu.RLock()
			rd := redisDriver
			mu.RUnlock()
			if rd == nil || lastDriver != limiter.DriverRedis {
				mu.Lock()
				if redisDriver == nil || lastDriver != limiter.DriverRedis {
					var client redis.Cmdable
					if data.RedisClient() != nil {
						client = data.RedisClient()
					}
					redisDriver = limiter.NewRedisDriver(client)
					lastDriver = limiter.DriverRedis
				}
				rd = redisDriver
				mu.Unlock()
			}
			allowed, _ := rd.Allow(c.Request.Context(), key, cfg.RPS, cfg.Burst)
			if !allowed {
				response.Resp().SetHttpCode(http.StatusTooManyRequests).Fail(c, http.StatusTooManyRequests, "Too Many Requests")
				return
			}
			c.Next()
			return
		}

		// memory 模式
		allowed, _ := memDriver.Allow(c.Request.Context(), key, cfg.RPS, cfg.Burst)
		if !allowed {
			response.Resp().SetHttpCode(http.StatusTooManyRequests).Fail(c, http.StatusTooManyRequests, "Too Many Requests")
			return
		}
		c.Next()
	}
}
