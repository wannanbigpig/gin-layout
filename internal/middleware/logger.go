package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

const (
	defaultStatusCode   = http.StatusOK
	maxRequestBodySize  = 16 * 1024 // 请求体最大记录大小：16KB
	maxResponseBodySize = 32 * 1024 // 响应体最大记录大小：32KB
)

// skipLogPaths 集中维护审计日志/响应记录需要跳过的路径，避免在多个判定点散落硬编码字符串。
var skipLogPaths = map[string]struct{}{
	"/ping": {},
}

// websocketLoggingPaths 标识在审计日志中应识别为 WebSocket 长连接的路径。
// 路径须与 internal/routers/admin_router.go 中 common 组注册的 "ws" 路由保持一致。
var websocketLoggingPaths = map[string]struct{}{
	"/admin/v1/common/ws": {},
}

// WebSocketAuditOptions 覆盖 WebSocket 连接审计的结果状态。
type WebSocketAuditOptions struct {
	OperationStatus int
	ResponseStatus  int
	ResponseBody    string
}

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
	if c == nil || c.Request == nil || recorder == nil {
		return
	}
	if _, ws := websocketLoggingPaths[c.Request.URL.Path]; ws {
		return
	}

	resp := parseResponse(c, recorder)
	snapshot := buildRequestAuditLogSnapshot(c, recorder, resp)
	recordRequestStat(jobs.AuditLogKindRequest, snapshot)

	if shouldSkipRequestLogging(c, recorder) {
		return
	}

	enqueueAuditLog(c, jobs.AuditLogKindRequest, snapshot)
}

// shouldSkipRequestLogging 判定是否跳过本次请求的日志记录。
func shouldSkipRequestLogging(c *gin.Context, recorder *responseRecorder) bool {
	if c == nil || c.Request == nil || recorder == nil {
		return true
	}

	// 跳过预设路径（如 /ping），避免高频探活请求污染审计日志。
	if _, skip := skipLogPaths[c.Request.URL.Path]; skip {
		return true
	}

	// WebSocket 已由 handler 在断开后主动调用 LogWebSocketConnection 记录（含连接时长）。
	// 中间件收尾路径若再记一条，会产生重复（且其 statusCode 停留在 hijack 前的默认 200）。
	if _, ws := websocketLoggingPaths[c.Request.URL.Path]; ws {
		return true
	}

	// 404 请求不记录日志（避免过多无效请求干扰日志分析）
	return recorder.statusCode == http.StatusNotFound
}

// LogWebSocketConnection 记录一次 WebSocket 连接的审计日志。
//
// WebSocket 经 gorilla upgrader.Upgrade → Hijack 接管底层连接，握手响应与后续帧
// 完全旁路 CustomLogger 的 responseRecorder；同时 handler 在连接存活期间阻塞，
// c.Next() 不返回，中间件无法收尾。故由 handler 在 ServeWebsocket 返回（连接断开）后主动调用本函数。
//
// 调用时机为断开后：执行时长经 calculateExecutionTimeMeta 按 WebSocket 类型计为
// 请求开始至断开的连接全程时长（ExecutionTimeScopeConnection，单位秒）。
// OperationStatus 取 0（业务成功，对应展示层"成功"）；ResponseStatus 取 HTTP 101
// （Switching Protocols，贴合真实握手语义，不参与成功/失败判定）。
// 采样丢弃由 shouldDropBySampling 对 WebSocket 类型显式放行。
func LogWebSocketConnection(c *gin.Context, opts ...WebSocketAuditOptions) {
	if c == nil || c.Request == nil {
		return
	}
	option := WebSocketAuditOptions{
		OperationStatus: 0,
		ResponseStatus:  http.StatusSwitchingProtocols,
	}
	if len(opts) > 0 {
		option = opts[0]
		if option.ResponseStatus == 0 {
			option.ResponseStatus = http.StatusSwitchingProtocols
		}
	}
	snapshot := buildAuditLogSnapshot(c, nil, option.OperationStatus, option.ResponseBody, "")
	if snapshot == nil {
		return
	}
	snapshot.RequestKind = model.RequestKindWebSocket
	snapshot.ResponseStatus = option.ResponseStatus
	recordRequestStat(jobs.AuditLogKindRequest, snapshot)
	enqueueAuditLog(c, jobs.AuditLogKindRequest, snapshot)
}
