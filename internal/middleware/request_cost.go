package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

// RequestCostHandler 请求耗时和请求ID中间件
// 功能：
// 1. 记录请求开始时间，用于后续计算请求耗时
// 2. 为每个请求生成唯一的请求ID，用于日志追踪
func RequestCostHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置请求上下文信息：开始时间 + 请求ID。
		c.Set(global.ContextKeyRequestStartTime, time.Now())
		c.Set(global.ContextKeyRequestID, uuid.New().String())
		c.Next()
	}
}
