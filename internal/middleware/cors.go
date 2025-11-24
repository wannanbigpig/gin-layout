package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/config"
)

var (
	// defaultMethods 默认允许的HTTP方法
	defaultMethods = []string{
		"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS",
	}
)

// CorsHandler 创建CORS跨域资源共享中间件
// 直接实现CORS处理，不依赖第三方库
// 所有配置项都可以通过 config.yaml 进行配置
func CorsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 设置 CORS 响应头
		if origin != "" {
			// 检查是否允许该源
			if isOriginAllowed(origin) {
				c.Header("Access-Control-Allow-Origin", origin)
			} else {
				// 如果不允许，不设置 Access-Control-Allow-Origin
				// 浏览器会拒绝该响应
				c.AbortWithStatus(403)
				return
			}

			// 允许的请求方法（从配置读取，如果未配置则使用默认值）
			methods := getAllowedMethods()
			c.Header("Access-Control-Allow-Methods", strings.Join(methods, ", "))

			// 允许的请求头（从配置读取）
			headers := getAllowedHeaders(c)
			c.Header("Access-Control-Allow-Headers", headers)

			// 暴露的响应头（从配置读取）
			exposeHeaders := getExposeHeaders()
			c.Header("Access-Control-Expose-Headers", exposeHeaders)

			// 预检请求缓存时间（从配置读取）
			maxAge := getMaxAge()
			c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", maxAge))

			// 是否允许携带凭证（从配置读取）
			// 注意：如果 AllowCredentials 为 true，Access-Control-Allow-Origin 不能为 *
			credentials := config.Config.CorsCredentials
			c.Header("Access-Control-Allow-Credentials", fmt.Sprintf("%t", credentials))
		}

		// 处理 OPTIONS 预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204) // No Content
			return
		}

		// 其他请求继续处理
		c.Next()
	}
}

// getAllowedMethods 获取允许的HTTP方法
func getAllowedMethods() []string {
	if len(config.Config.CorsMethods) > 0 {
		return config.Config.CorsMethods
	}
	return defaultMethods
}

// getAllowedHeaders 获取允许的请求头
func getAllowedHeaders(c *gin.Context) string {
	// 如果配置了允许的请求头列表
	if len(config.Config.CorsHeaders) > 0 {
		return strings.Join(config.Config.CorsHeaders, ", ")
	}

	// 如果未配置，检查预检请求中的 Access-Control-Request-Headers
	requestHeaders := c.Request.Header.Get("Access-Control-Request-Headers")
	if requestHeaders != "" {
		return requestHeaders
	}

	// 默认允许所有请求头
	return "*"
}

// getExposeHeaders 获取暴露的响应头
func getExposeHeaders() string {
	if len(config.Config.CorsExposeHeaders) > 0 {
		return strings.Join(config.Config.CorsExposeHeaders, ", ")
	}
	// 默认暴露所有响应头
	return "*"
}

// getMaxAge 获取预检请求缓存时间（秒）
func getMaxAge() int {
	if config.Config.CorsMaxAge > 0 {
		return config.Config.CorsMaxAge
	}
	// 默认 12 小时
	return 43200
}

// isOriginAllowed 检查源是否被允许
func isOriginAllowed(origin string) bool {
	// 如果配置了允许的源列表，检查是否在列表中
	if len(config.Config.CorsOrigins) > 0 {
		for _, allowedOrigin := range config.Config.CorsOrigins {
			if origin == allowedOrigin {
				return true
			}
			// 支持通配符匹配（如 http://*.example.com）
			if strings.Contains(allowedOrigin, "*") {
				if matchWildcard(origin, allowedOrigin) {
					return true
				}
			}
		}
		return false
	}

	// 如果没有配置允许的源列表，默认允许所有源
	return true
}

// matchWildcard 通配符匹配
func matchWildcard(origin, pattern string) bool {
	// 简单的通配符匹配实现
	// 支持 * 在开头、结尾或中间
	if pattern == "*" {
		return true
	}

	// 开头通配符：*.example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*.")
		return strings.HasSuffix(origin, suffix)
	}

	// 结尾通配符：http://example.*
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(origin, prefix)
	}

	// 中间通配符：http://*.example.com
	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		return strings.HasPrefix(origin, parts[0]) && strings.HasSuffix(origin, parts[1])
	}

	return false
}
