package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestCostHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置请求开始时间
		c.Set("requestStartTime", time.Now())
		// 设置请求ID,生成一个uuid
		c.Set("requestId", uuid.New().String())
		c.Next()

	}
}
