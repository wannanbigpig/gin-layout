package middleware

import (
	"fmt"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/config"
)

var (
	defaultMethods = []string{
		"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS",
	}
)

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
			return defaultMethods
		}
		return cfg.CorsMethods
	}
	return defaultMethods
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
	if len(cfg.CorsOrigins) == 0 {
		return false
	}
	for _, allowedOrigin := range cfg.CorsOrigins {
		if allowedOrigin == "*" {
			return true
		}
		if origin == allowedOrigin {
			return true
		}
		if strings.Contains(allowedOrigin, "*") {
			if matched, _ := path.Match(allowedOrigin, origin); matched {
				return true
			}
		}
	}
	return false
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
