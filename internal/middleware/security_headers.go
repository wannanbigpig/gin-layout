package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersHandler 添加 HTTP 安全响应头。
func SecurityHeadersHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-XSS-Protection", "0")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// 仅 HTTPS 时启用 HSTS
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}
