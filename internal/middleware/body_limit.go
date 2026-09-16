package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/config"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

// RequestBodyLimitHandler 在绑定和日志读取前限制实体大小，也覆盖 chunked 请求。
func RequestBodyLimitHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		limitMB := int64(16)
		if cfg := config.GetConfig(); cfg != nil && cfg.MaxRequestBodyMB > 0 {
			limitMB = cfg.MaxRequestBodyMB
		}
		if c.Request.Method == http.MethodPost && (c.FullPath() == "/admin/v1/system/file/upload/local" || c.FullPath() == "/admin/v1/common/upload") {
			limitMB = 256
			if cfg := config.GetConfig(); cfg != nil && cfg.MaxUploadBodyMB > 0 {
				limitMB = cfg.MaxUploadBodyMB
			}
		}
		// 限制配置换算，避免溢出导致负数限制。
		if limitMB > 1<<30 {
			limitMB = 1 << 30
		}
		limit := limitMB << 20
		if c.Request.ContentLength > limit {
			response.Resp().SetHttpCode(http.StatusRequestEntityTooLarge).FailCode(c, e.InvalidParameter)
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}
