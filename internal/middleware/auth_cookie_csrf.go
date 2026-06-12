package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

const csrfRequestedWithValue = "XMLHttpRequest"

// AuthCookieCSRFGuard 为仅依赖 HttpOnly Cookie 的认证接口提供最小 CSRF 防护。
func AuthCookieCSRFGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(c.GetHeader("X-Requested-With")) != csrfRequestedWithValue {
			response.FailCode(c, e.AuthorizationErr)
			c.Abort()
			return
		}
		c.Next()
	}
}
