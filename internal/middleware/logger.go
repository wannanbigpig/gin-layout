package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

type responseRecorder struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteString(s string) (int, error) {
	r.body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// CustomLogger 接收gin框架默认的日志
func CustomLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 创建响应记录器
		recorder := &responseRecorder{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(nil),
			statusCode:     http.StatusOK, // 默认状态码
		}
		c.Writer = recorder

		// 处理请求
		c.Next()

		// 计算耗时
		cost := time.Since(c.GetTime("requestStartTime"))

		// 获取请求ID
		requestID := c.GetString("requestId")
		path := c.Request.URL.Path

		// 解析响应
		var resp response.Result
		if err := json.Unmarshal(recorder.body.Bytes(), &resp); err == nil {
			// 如果是GET请求，不记录数据部分
			if c.Request.Method == http.MethodGet {
				resp.Data = nil
			}
		}

		// 记录日志
		logFields := []zap.Field{
			zap.String("requestId", requestID),
			zap.Int("uid", c.GetInt("uid")),
			zap.Int("status", recorder.statusCode),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Duration("cost", cost),
		}

		// 只有在非生产环境才记录请求体和响应体
		if gin.Mode() != gin.ReleaseMode {
			// 如果有请求体可以在这里添加
			if body, err := c.GetRawData(); err == nil {
				logFields = append(logFields, zap.ByteString("body", body))
			}

			respBody, _ := json.Marshal(resp)
			logFields = append(logFields, zap.String("response", string(respBody)))
		}

		log.Info(path, logFields...)
	}
}
