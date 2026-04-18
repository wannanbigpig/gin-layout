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
	"github.com/wannanbigpig/gin-layout/internal/jobs"
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
	errStr := "服务器内部错误"
	if config.Config.Debug {
		errStr = fmt.Sprintf("%v", err)
	}

	// 记录错误日志
	cost := time.Since(c.GetTime(global.ContextKeyRequestStartTime))
	requestID := c.GetString(global.ContextKeyRequestID)
	logFields := []zap.Field{
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
	if gin.Mode() != gin.ReleaseMode {
		if requestBody := readRequestBody(c); requestBody != nil {
			logFields = append(logFields, zap.ByteString("body", requestBody))
		}
	}
	logger.Error(panicRecoveredMsg, logFields...)

	// 为 panic 请求补充异步审计日志，避免绕过 CustomLogger 的落库流程。
	snapshot := buildPanicAuditLogSnapshot(c, errStr)
	enqueueAuditLog(c, jobs.AuditLogKindPanic, snapshot)

	// 返回错误响应
	response.Resp().
		SetHttpCode(http.StatusInternalServerError).
		FailCode(c, e.ServerErr, errStr)
}

// PanicExceptionRecord panic 异常记录器
// 实现 io.Writer 接口，用于记录 panic 的完整堆栈信息
type PanicExceptionRecord struct{}

// Write 写入 panic 异常信息
func (p *PanicExceptionRecord) Write(b []byte) (n int, err error) {
	var builder strings.Builder
	builder.Grow(len(panicErrorPrefix) + len(b))
	builder.WriteString(panicErrorPrefix)
	builder.Write(b)
	errStr := builder.String()
	logger.Error(errStr)
	return len(errStr), errors.New(errStr)
}
