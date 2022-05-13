package middleware

import (
	"github.com/gin-gonic/gin"
	"time"
)

func RequestCostHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("requestStartTime", time.Now())
	}
}
