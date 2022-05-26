package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	log "github.com/wannanbigpig/gin-layout/pkg/logger"
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
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()

		cost := time.Since(c.GetTime("requestStartTime"))

		log.Logger.Info(path,
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.String("cost", cost.String()),
			zap.String("response", blw.body.String()),
		)
	}
}
