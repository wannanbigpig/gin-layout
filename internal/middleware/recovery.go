package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

const (
	// panicErrorPrefix 服务器内部错误前缀
	panicErrorPrefix = "An error occurred in the server's internal code: "
	// panicRecoveredMsg panic恢复日志消息
	panicRecoveredMsg = "panic recovered"
)

// CustomRecovery 自定义错误 (panic) 拦截中间件
// 对可能发生的 panic 进行拦截、统一记录并返回友好的错误响应
func CustomRecovery() gin.HandlerFunc {
	errorWriter := &PanicExceptionRecord{}
	return gin.RecoveryWithWriter(errorWriter, handlePanic)
}

// handlePanic 处理 panic 恢复逻辑
func handlePanic(c *gin.Context, err interface{}) {
	// 格式化错误信息
	errStr := formatError(err)

	// 记录错误日志
	logPanicError(c, errStr)

	// 为 panic 请求补充数据库审计日志，避免绕过 CustomLogger 的落库流程。
	go savePanicRequestLogToDB(c, errStr)

	// 返回错误响应
	sendErrorResponse(c, errStr)
}

// formatError 格式化错误信息
func formatError(err interface{}) string {
	if !config.Config.Debug {
		return ""
	}
	return fmt.Sprintf("%v", err)
}

// logPanicError 记录 panic 错误日志
func logPanicError(c *gin.Context, errStr string) {
	// 构建基础日志字段
	logFields := buildPanicLogFields(c, errStr)

	// 添加调试信息（非生产环境）
	if gin.Mode() != gin.ReleaseMode {
		logFields = appendPanicDebugFields(c, logFields)
	}

	// 记录错误日志
	logger.Error(panicRecoveredMsg, logFields...)
}

// buildPanicLogFields 构建 panic 日志字段
func buildPanicLogFields(c *gin.Context, errStr string) []zap.Field {
	cost := time.Since(c.GetTime(global.ContextKeyRequestStartTime))
	requestID := c.GetString(global.ContextKeyRequestID)

	return []zap.Field{
		zap.String("requestId", requestID),
		zap.Int("status", c.Writer.Status()),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("query", c.Request.URL.RawQuery),
		zap.String("ip", c.ClientIP()),
		zap.String("user-agent", c.Request.UserAgent()),
		zap.String("errors", errStr),
		zap.Duration("cost", cost),
	}
}

// appendPanicDebugFields 添加 panic 调试字段（仅非生产环境）
func appendPanicDebugFields(c *gin.Context, logFields []zap.Field) []zap.Field {
	// 读取请求体（如果存在且未被消耗）
	if requestBody := readRequestBody(c); requestBody != nil {
		logFields = append(logFields, zap.ByteString("body", requestBody))
	}
	return logFields
}

// sendErrorResponse 发送错误响应
func sendErrorResponse(c *gin.Context, errStr string) {
	response.Resp().
		SetHttpCode(http.StatusInternalServerError).
		FailCode(c, e.ServerErr, errStr)
}

// PanicExceptionRecord panic 异常记录器
// 实现 io.Writer 接口，用于记录 panic 的完整堆栈信息
type PanicExceptionRecord struct{}

// Write 写入 panic 异常信息
func (p *PanicExceptionRecord) Write(b []byte) (n int, err error) {
	errStr := buildPanicErrorString(b)
	logger.Error(errStr)
	return len(errStr), errors.New(errStr)
}

// buildPanicErrorString 构建 panic 错误字符串
func buildPanicErrorString(stackTrace []byte) string {
	var builder strings.Builder
	builder.Grow(len(panicErrorPrefix) + len(stackTrace))
	builder.WriteString(panicErrorPrefix)
	builder.Write(stackTrace)
	return builder.String()
}
