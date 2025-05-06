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
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

// CustomRecovery 自定义错误 (panic) 拦截中间件、对可能发生的错误进行拦截、统一记录
func CustomRecovery() gin.HandlerFunc {
	DefaultErrorWriter := &PanicExceptionRecord{}
	return gin.RecoveryWithWriter(DefaultErrorWriter, func(c *gin.Context, err interface{}) {
		// 记录错误信息
		errStr := ""
		if config.Config.Debug {
			errStr = fmt.Sprintf("%v", err)
		}
		// 记录请求日志
		cost := time.Since(c.GetTime("requestStartTime"))
		requestId := c.GetString("requestId")
		path := c.Request.URL.Path

		// 构建日志字段
		logFields := []zap.Field{
			zap.String("requestId", requestId),
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", errStr),
			zap.Duration("cost", cost),
		}
		// 只有在非生产环境才记录请求体和响应体
		if gin.Mode() != gin.ReleaseMode {
			// 如果有请求体可以在这里添加
			if body, err := c.GetRawData(); err == nil {
				logFields = append(logFields, zap.ByteString("body", body))
			}
		}

		// 记录错误日志
		logger.Error("panic recovered", logFields...)

		// 返回错误响应
		response.Resp().SetHttpCode(http.StatusInternalServerError).FailCode(c, e.ServerErr, errStr)
	})
}

// PanicExceptionRecord  panic等异常记录
type PanicExceptionRecord struct{}

func (p *PanicExceptionRecord) Write(b []byte) (n int, err error) {
	// 构建错误信息
	s1 := "An error occurred in the server's internal code: "
	var build strings.Builder
	build.WriteString(s1)
	build.Write(b)
	errStr := build.String()

	// 记录完整的错误堆栈
	logger.Error(errStr)

	return len(errStr), errors.New(errStr)
}
