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
		// 设置请求上下文信息
		setRequestContext(c)
		// 暂停1-5秒随机时间
		// randomSeconds := rand.Intn(5) + 1 // 生成1-5的随机数
		// time.Sleep(time.Duration(randomSeconds) * time.Second)
		c.Next()
	}
}

// setRequestContext 设置请求上下文信息
func setRequestContext(c *gin.Context) {
	// 设置请求开始时间
	c.Set(global.ContextKeyRequestStartTime, time.Now())

	// 生成并设置请求ID
	requestID := generateRequestID()
	c.Set(global.ContextKeyRequestID, requestID)
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return uuid.New().String()
}
