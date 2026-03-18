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
		cfg := config.GetConfig()
		origin := c.Request.Header.Get("Origin")

		// 处理 OPTIONS 预检请求（需要在检查 origin 之前处理）
		if c.Request.Method == "OPTIONS" {
			if origin != "" {
				// 检查是否允许该源
				if isOriginAllowed(origin) {
					c.Header("Access-Control-Allow-Origin", origin)

					// 允许的请求方法（从配置读取，如果未配置则使用默认值）
					methods := getAllowedMethods()
					c.Header("Access-Control-Allow-Methods", strings.Join(methods, ", "))

					// 允许的请求头（从配置读取）
					headers := getAllowedHeaders(c)
					c.Header("Access-Control-Allow-Headers", headers)

					// 预检请求缓存时间（从配置读取）
					maxAge := getMaxAge()
					c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", maxAge))

					// 是否允许携带凭证（从配置读取）
					// 注意：如果 AllowCredentials 为 true，Access-Control-Allow-Origin 不能为 *
					credentials := cfg.CorsCredentials
					c.Header("Access-Control-Allow-Credentials", fmt.Sprintf("%t", credentials))
				} else {
					// 如果不允许，返回 403
					c.AbortWithStatus(403)
					return
				}
			}
			c.AbortWithStatus(204) // No Content
			return
		}

		// 设置 CORS 响应头（对于非 OPTIONS 请求）
		if origin != "" {
			// 检查是否允许该源
			if isOriginAllowed(origin) {
				c.Header("Access-Control-Allow-Origin", origin)

				// 暴露的响应头（从配置读取）
				exposeHeaders := getExposeHeaders()
				c.Header("Access-Control-Expose-Headers", exposeHeaders)

				// 是否允许携带凭证（从配置读取）
				// 注意：如果 AllowCredentials 为 true，Access-Control-Allow-Origin 不能为 *
				credentials := cfg.CorsCredentials
				c.Header("Access-Control-Allow-Credentials", fmt.Sprintf("%t", credentials))
			} else {
				// 如果不允许，不设置 Access-Control-Allow-Origin
				// 浏览器会拒绝该响应
				c.AbortWithStatus(403)
				return
			}
		}

		// 其他请求继续处理
		c.Next()
	}
}

// getAllowedMethods 获取允许的HTTP方法
func getAllowedMethods() []string {
	cfg := config.GetConfig()
	if len(cfg.CorsMethods) > 0 {
		return cfg.CorsMethods
	}
	return defaultMethods
}

// getAllowedHeaders 获取允许的请求头
func getAllowedHeaders(c *gin.Context) string {
	// 如果配置了允许的请求头列表
	cfg := config.GetConfig()
	if len(cfg.CorsHeaders) > 0 {
		return strings.Join(cfg.CorsHeaders, ", ")
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
	cfg := config.GetConfig()
	if len(cfg.CorsExposeHeaders) > 0 {
		return strings.Join(cfg.CorsExposeHeaders, ", ")
	}
	// 默认暴露所有响应头
	return "*"
}

// getMaxAge 获取预检请求缓存时间（秒）
func getMaxAge() int {
	cfg := config.GetConfig()
	if cfg.CorsMaxAge > 0 {
		return cfg.CorsMaxAge
	}
	// 默认 12 小时
	return 43200
}

// isOriginAllowed 检查源是否被允许
func isOriginAllowed(origin string) bool {
	cfg := config.GetConfig()
	// 如果配置了允许的源列表，检查是否在列表中
	if len(cfg.CorsOrigins) > 0 {
		for _, allowedOrigin := range cfg.CorsOrigins {
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

	// 提取协议和域名部分
	var originProtocol, originDomain string
	var patternProtocol, patternDomain string

	if strings.Contains(origin, "://") {
		originParts := strings.Split(origin, "://")
		if len(originParts) == 2 {
			originProtocol = originParts[0]
			originDomain = originParts[1]
		} else {
			originDomain = origin
		}
	} else {
		originDomain = origin
	}

	if strings.Contains(pattern, "://") {
		patternParts := strings.Split(pattern, "://")
		if len(patternParts) == 2 {
			patternProtocol = patternParts[0]
			patternDomain = patternParts[1]
		} else {
			patternDomain = pattern
		}
	} else {
		patternDomain = pattern
	}

	// 如果模式指定了协议，则协议必须匹配
	if patternProtocol != "" && originProtocol != patternProtocol {
		return false
	}

	// 对域名部分进行通配符匹配
	return matchDomainWildcard(originDomain, patternDomain)
}

// matchDomainWildcard 匹配域名部分的通配符（不包含协议）
func matchDomainWildcard(originDomain, patternDomain string) bool {
	// 开头通配符：*.example.com（匹配 example.com 的所有子域名）
	if strings.HasPrefix(patternDomain, "*.") {
		suffix := strings.TrimPrefix(patternDomain, "*.")
		// 检查是否以指定后缀结尾，且不是完全相等（确保是子域名）
		if strings.HasSuffix(originDomain, suffix) && originDomain != suffix {
			return true
		}
		return false
	}

	// 结尾通配符：example.*
	if strings.HasSuffix(patternDomain, ".*") {
		prefix := strings.TrimSuffix(patternDomain, ".*")
		return strings.HasPrefix(originDomain, prefix)
	}

	// 中间通配符：*.example.com 或 sub.*.example.com
	parts := strings.Split(patternDomain, "*")
	if len(parts) == 2 {
		return strings.HasPrefix(originDomain, parts[0]) && strings.HasSuffix(originDomain, parts[1])
	}

	return false
}
