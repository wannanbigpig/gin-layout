package middleware

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/internal/pkg/error_code"
	response2 "github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/pkg/logger"
	"go.uber.org/zap"
	"net/http"
)

// CustomRecovery 自定义错误 (panic) 拦截中间件、对可能发生的错误进行拦截、统一记录
func CustomRecovery() gin.HandlerFunc {
	DefaultErrorWriter := &PanicExceptionRecord{}
	return gin.RecoveryWithWriter(DefaultErrorWriter, func(c *gin.Context, err interface{}) {
		// 这里针对发生的panic等异常进行统一响应即可
		response2.NewResponse().SetHttpCode(http.StatusInternalServerError).FailCode(c, error_code.ServerError)
	})
}

//PanicExceptionRecord  panic等异常记录
type PanicExceptionRecord struct{}

func (p *PanicExceptionRecord) Write(b []byte) (n int, err error) {
	errStr := string(b)
	err = errors.New(errStr)
	logger.Logger.Error("服务器内部代码发生错误。", zap.String("msg", errStr))
	return len(errStr), err
}
