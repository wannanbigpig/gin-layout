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
	if shouldSkipRequestLogging(c, recorder) {
		return
	}

	// 记录精简日志到文件（用于快速查看和调试）
	logRequestToFile(c, recorder)

	dispatchRequestAuditLog(c, recorder)
}

func shouldSkipRequestLogging(c *gin.Context, recorder *responseRecorder) bool {
	if c == nil || c.Request == nil || recorder == nil {
		return true
	}

	// ping 请求不记录日志
	if c.Request.URL.Path == "/ping" {
		return true
	}

	// 404 请求不记录日志（避免过多无效请求干扰日志分析）
	return recorder.statusCode == http.StatusNotFound
}

func dispatchRequestAuditLog(c *gin.Context, recorder *responseRecorder) {
	// 先提取不可变快照，再发布审计日志，避免在日志中间件内耦合过多细节。
	resp := parseResponse(c, recorder)
	snapshot := buildRequestAuditLogSnapshot(c, recorder, resp)
	enqueueAuditLog(c, jobs.AuditLogKindRequest, snapshot)
}

// logRequestToFile 记录精简日志到文件（用于快速查看和调试，仅记录错误请求）。
// 错误请求定义：HTTP 状态码 >= 400，或存在 private 错误。
func logRequestToFile(c *gin.Context, recorder *responseRecorder) {
	if c == nil || c.Request == nil || recorder == nil {
		return
	}

	privateErrors := c.Errors.ByType(gin.ErrorTypePrivate)
	if recorder.statusCode < http.StatusBadRequest && len(privateErrors) == 0 {
		return
	}

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

	// 如果有 private 错误，记录错误信息。
	if len(privateErrors) > 0 {
		logFields = append(logFields, zap.String("errors", privateErrors.String()))
	}

	// 记录日志到文件
	log.Info(c.Request.URL.Path, logFields...)
}
