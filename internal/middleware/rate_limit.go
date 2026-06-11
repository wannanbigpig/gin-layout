package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Enabled bool
	RPS     float64
	Burst   int
	Name    string
}

// ipLimiter 每 IP 限流器
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimitHandler 全局 IP 级速率限制中间件。
// 基于内存令牌桶实现，适用于单实例部署场景。
// 配置关闭时直接透传，不影响已有行为。
func RateLimitHandler(cfg RateLimitConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	if cfg.RPS <= 0 {
		cfg.RPS = 1
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 1
	}

	var (
		mu       sync.Mutex
		visitors = make(map[string]*ipLimiter)
	)

	// 后台清理过期条目，避免内存泄漏
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, v := range visitors {
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()

	getVisitor := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()

		v, exists := visitors[ip]
		if !exists {
			limiter := rate.NewLimiter(rate.Limit(cfg.RPS), cfg.Burst)
			visitors[ip] = &ipLimiter{limiter: limiter, lastSeen: time.Now()}
			return limiter
		}
		v.lastSeen = time.Now()
		return v.limiter
	}

	return func(c *gin.Context) {
		limiter := getVisitor(c.ClientIP())
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    http.StatusTooManyRequests,
				"message": "Too Many Requests",
			})
			return
		}
		c.Next()
	}
}
