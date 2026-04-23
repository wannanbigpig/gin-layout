package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/jobs"
)

const (
	defaultStatusCode   = http.StatusOK
	maxRequestBodySize  = 16 * 1024 // 请求体最大记录大小：16KB
	maxResponseBodySize = 32 * 1024 // 响应体最大记录大小：32KB
)

// CustomLogger 自定义日志中间件
func CustomLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		recorder := prepareRequestLogging(c)
		c.Next()
		finalizeRequestLogging(c, recorder)
	}
}

// prepareRequestLogging 在业务处理前完成日志采集准备（请求体快照 + 响应录制器）。
func prepareRequestLogging(c *gin.Context) *responseRecorder {
	// 在请求处理前缓存请求体，避免后续读取时丢失原始内容。
	cacheRequestBody(c)

	// 替换 Writer 以捕获响应状态码和响应体快照。
	recorder := createResponseRecorder(c)
	c.Writer = recorder
	return recorder
}

// finalizeRequestLogging 在业务处理后统一执行日志收尾（仅审计日志）。
func finalizeRequestLogging(c *gin.Context, recorder *responseRecorder) {
	if shouldSkipRequestLogging(c, recorder) {
		return
	}

	publishRequestAuditLog(c, recorder)
}

// logRequest 兼容旧调用入口，统一委托到收尾阶段。
func logRequest(c *gin.Context, recorder *responseRecorder) {
	finalizeRequestLogging(c, recorder)
}

// shouldSkipRequestLogging 判定是否跳过本次请求的日志记录。
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

// publishRequestAuditLog 构建请求审计快照并投递到审计日志链路。
func publishRequestAuditLog(c *gin.Context, recorder *responseRecorder) {
	// 先提取不可变快照，再发布审计日志，避免在日志中间件内耦合过多细节。
	resp := parseResponse(c, recorder)
	snapshot := buildRequestAuditLogSnapshot(c, recorder, resp)
	enqueueAuditLog(c, jobs.AuditLogKindRequest, snapshot)
}
