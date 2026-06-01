package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/pkg/corsutil"
)

var builtInDefaultMethods = [...]string{
	"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS",
}

// ValidateCORSConfig 校验 CORS 配置的合法性。
// cors_origins 包含 "*" 且 cors_credentials 为 true 是安全风险（浏览器不允许通配符 origin + credentials），
// 启动时应检测并告警。
func ValidateCORSConfig(cfg *config.Conf) {
	if cfg == nil {
		return
	}
	if cfg.CorsCredentials && hasWildcardOrigin(cfg) {
		panic("CORS 配置冲突：cors_origins 包含通配符 \"*\" 且 cors_credentials 为 true，" +
			"浏览器不允许同时使用通配符 origin 和携带凭证，请修改配置")
	}
}

func CorsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.GetConfig()
		origin := c.Request.Header.Get("Origin")

		if c.Request.Method == "OPTIONS" {
			if origin != "" {
				if allowOrigin, ok := resolveAllowOrigin(origin, cfg); ok {
					c.Header("Access-Control-Allow-Origin", allowOrigin)
					c.Header("Access-Control-Allow-Methods", strings.Join(getAllowedMethods(cfg), ", "))
					c.Header("Access-Control-Allow-Headers", getAllowedHeaders(c, cfg))
					c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", getMaxAge(cfg)))
					c.Header("Access-Control-Allow-Credentials", fmt.Sprintf("%t", cfg.CorsCredentials))
				} else {
					c.AbortWithStatus(403)
					return
				}
			}
			c.AbortWithStatus(204)
			return
		}

		if origin != "" {
			if allowOrigin, ok := resolveAllowOrigin(origin, cfg); ok {
				c.Header("Access-Control-Allow-Origin", allowOrigin)
				c.Header("Access-Control-Expose-Headers", getExposeHeaders(cfg))
				c.Header("Access-Control-Allow-Credentials", fmt.Sprintf("%t", cfg.CorsCredentials))
			} else {
				c.AbortWithStatus(403)
				return
			}
		}

		c.Next()
	}
}

func resolveAllowOrigin(origin string, cfg *config.Conf) (string, bool) {
	if !isOriginAllowed(origin, cfg) {
		return "", false
	}
	if hasWildcardOrigin(cfg) && !cfg.CorsCredentials {
		return "*", true
	}
	return origin, true
}

func getAllowedMethods(cfg *config.Conf) []string {
	if len(cfg.CorsMethods) > 0 {
		if hasWildcardValue(cfg.CorsMethods) {
			return defaultMethods()
		}
		return cfg.CorsMethods
	}
	return defaultMethods()
}

func defaultMethods() []string {
	methods := make([]string, len(builtInDefaultMethods))
	copy(methods, builtInDefaultMethods[:])
	return methods
}

func getAllowedHeaders(c *gin.Context, cfg *config.Conf) string {
	if len(cfg.CorsHeaders) > 0 {
		if hasWildcardValue(cfg.CorsHeaders) {
			requestHeaders := c.Request.Header.Get("Access-Control-Request-Headers")
			if requestHeaders != "" {
				return requestHeaders
			}
			return "*"
		}
		return strings.Join(cfg.CorsHeaders, ", ")
	}
	requestHeaders := c.Request.Header.Get("Access-Control-Request-Headers")
	if requestHeaders != "" {
		return requestHeaders
	}
	return "*"
}

func getExposeHeaders(cfg *config.Conf) string {
	if len(cfg.CorsExposeHeaders) > 0 {
		if hasWildcardValue(cfg.CorsExposeHeaders) {
			return "*"
		}
		return strings.Join(cfg.CorsExposeHeaders, ", ")
	}
	return "*"
}

func getMaxAge(cfg *config.Conf) int {
	if cfg.CorsMaxAge > 0 {
		return cfg.CorsMaxAge
	}
	return 43200
}

func isOriginAllowed(origin string, cfg *config.Conf) bool {
	// HTTP CORS：无凭证时允许字面 "*"，沿用历史行为。
	return corsutil.IsOriginAllowed(origin, cfg.CorsOrigins, true)
}

func hasWildcardOrigin(cfg *config.Conf) bool {
	for _, allowedOrigin := range cfg.CorsOrigins {
		if allowedOrigin == "*" {
			return true
		}
	}
	return false
}

func hasWildcardValue(values []string) bool {
	for _, value := range values {
		if value == "*" {
			return true
		}
	}
	return false
}
