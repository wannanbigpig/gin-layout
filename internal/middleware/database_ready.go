package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

// DatabaseReadyGuard 强制要求 MySQL 已就绪，否则直接返回统一业务错误。
func DatabaseReadyGuard() gin.HandlerFunc {
	return databaseReadyGuard(false)
}

// OptionalDatabaseReadyGuard 仅在请求已识别出登录用户时校验 MySQL 就绪状态。
// 用于需要先保留“未登录”语义的受保护路由。
func OptionalDatabaseReadyGuard() gin.HandlerFunc {
	return databaseReadyGuard(true)
}

func databaseReadyGuard(skipWhenNoUID bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if skipWhenNoUID && c.GetUint(global.ContextKeyUID) == 0 {
			c.Next()
			return
		}

		if data.MysqlReady() {
			c.Next()
			return
		}

		response.FailCode(c, e.ServiceDependencyNotReady)
	}
}
