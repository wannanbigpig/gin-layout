package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
	"time"
)

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w responseWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// CustomLogger 接收gin框架默认的日志
func CustomLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		blw := &responseWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw
		// 读取body数据
		//body := request.GetBody(c)
		c.Next()

		cost := time.Since(c.GetTime("requestStartTime"))
		if config.Config.AppEnv != "production" {
			path := c.Request.URL.Path
			log.Logger.Info(path,
				zap.Int("status", c.Writer.Status()),
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", c.Request.URL.RawQuery),
				//zap.Any("body", string(body)),
				zap.String("ip", c.ClientIP()),
				zap.String("user-agent", c.Request.UserAgent()),
				zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
				zap.String("cost", cost.String()),
				zap.String("response", blw.body.String()),
			)
		}
	}
}
