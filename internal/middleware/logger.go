package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

const (
	defaultStatusCode   = http.StatusOK
	maxRequestBodySize  = 16 * 1024 // 请求体最大记录大小：16KB
	maxResponseBodySize = 32 * 1024 // 响应体最大记录大小：32KB
)

// CustomLogger 自定义日志中间件
func CustomLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 在请求处理前读取并缓存请求体（避免后续处理消耗后无法读取）
		cacheRequestBody(c)

		// 创建响应记录器并替换 c.Writer（这样才能捕获响应）
		recorder := createResponseRecorder(c)
		c.Writer = recorder

		// 处理请求
		c.Next()

		// 记录请求日志
		logRequest(c, recorder)
	}
}

// logRequest 记录请求日志
func logRequest(c *gin.Context, recorder *responseRecorder) {
	// ping 请求不记录日志
	if c.Request.URL.Path == "/ping" {
		return
	}

	// 解析响应数据
	resp := parseResponse(c, recorder)

	// 记录精简日志到文件（用于快速查看和调试）
	logRequestToFile(c, recorder)

	// 先提取不可变快照，再发布异步任务，避免请求链路内直接落库。
	snapshot := buildRequestAuditLogSnapshot(c, recorder, resp)
	enqueueAuditLog(c, jobs.AuditLogKindRequest, snapshot)
}

// logRequestToFile 记录精简日志到文件（用于快速查看和调试）
func logRequestToFile(c *gin.Context, recorder *responseRecorder) {
	requestID := c.GetString(global.ContextKeyRequestID)
	if requestID == "" {
		return // 如果没有请求ID，不记录日志
	}

	cost := time.Since(c.GetTime(global.ContextKeyRequestStartTime))
	uid := c.GetUint(global.ContextKeyUID)

	logFields := []zap.Field{
		zap.String("requestId", requestID), // 关联字段，用于关联数据库日志
		zap.Uint("uid", uid),
		zap.Int("status", recorder.statusCode),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("ip", c.ClientIP()),
		zap.Duration("cost", cost),
	}

	// 如果有错误，记录错误信息
	if errors := c.Errors.ByType(gin.ErrorTypePrivate).String(); errors != "" {
		logFields = append(logFields, zap.String("errors", errors))
	}

	// 记录日志到文件
	log.Info(c.Request.URL.Path, logFields...)
}
