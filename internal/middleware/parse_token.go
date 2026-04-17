package middleware

import (
	"github.com/gin-gonic/gin"

	req "github.com/wannanbigpig/gin-layout/internal/pkg/request"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

// ParseTokenHandler 全局token解析中间件（所有路由都走）
// 功能：
//   - 尝试从请求头提取token（不强制要求）
//   - 如果token存在且有效，解析并设置用户信息到context
//   - 如果token不存在或无效，静默继续执行（用于可选认证的路由）
//
// 注意：此中间件不会阻止请求，即使token无效也会继续执行
func ParseTokenHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 提前返回：如果没有token，直接继续执行
		accessToken, err := req.GetAccessToken(c)
		if err != nil || accessToken == "" {
			c.Next()
			return
		}

		loginService := auth.NewLoginService()
		loginService.SetCtx(c)
		principal, ok := loginService.ResolvePrincipal(accessToken)
		if !ok || principal == nil {
			// token无效，静默继续（可选认证）
			c.Next()
			return
		}

		// token有效，设置认证主体到上下文
		auth.StoreAuthPrincipal(c, principal)
		c.Next()
	}
}
