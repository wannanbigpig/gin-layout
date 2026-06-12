package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/wannanbigpig/gin-layout/config/autoload"
)

func setupRateLimitRouter(cfg RateLimitConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimitHandler(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func TestRateLimit_Disabled(t *testing.T) {
	r := setupRateLimitRouter(RateLimitConfig{Enabled: false, RPS: 1, Burst: 1})

	// 连续请求均应通过，限流不生效
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass when disabled", i)
	}
}

func TestRateLimit_DefaultConfigEnabled(t *testing.T) {
	assert.True(t, autoload.App.RateLimitEnabled)
	assert.Equal(t, float64(20), autoload.App.RateLimitRPS)
	assert.Equal(t, 40, autoload.App.RateLimitBurst)
	assert.True(t, autoload.App.LoginRateLimitEnabled)
	assert.Equal(t, float64(1), autoload.App.LoginRateLimitRPS)
	assert.Equal(t, 5, autoload.App.LoginRateLimitBurst)
}

func TestRateLimit_Enabled_WithinLimit(t *testing.T) {
	r := setupRateLimitRouter(RateLimitConfig{Enabled: true, RPS: 10, Burst: 10})

	// 请求数未超过 burst，应全部通过
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass within limit", i)
	}
}

func TestRateLimit_Enabled_ExceedLimit(t *testing.T) {
	r := setupRateLimitRouter(RateLimitConfig{Enabled: true, RPS: 1, Burst: 1})

	// 第一个请求应通过
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 第二个请求应返回 429
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "Too Many Requests")
}

func TestRateLimit_LoginPolicyCanBeDisabled(t *testing.T) {
	r := setupRateLimitRouter(RateLimitConfig{Enabled: false, RPS: 1, Burst: 1, Name: "login"})

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass when login limit is disabled", i)
	}
}

func TestRateLimit_DifferentIPs(t *testing.T) {
	r := setupRateLimitRouter(RateLimitConfig{Enabled: true, RPS: 1, Burst: 1})

	// IP-A 第一个请求通过
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// IP-A 第二个请求被限流
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// IP-B 第一个请求应通过（不同 IP 互不影响）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.2")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_WithContextCanBeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := gin.New()
	r.Use(RateLimitHandlerWithContext(ctx, RateLimitConfig{Enabled: true, RPS: 1, Burst: 1}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	cancel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
