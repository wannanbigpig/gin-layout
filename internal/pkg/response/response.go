package response

import (
	"github.com/gin-gonic/gin"
	r "github.com/wannanbigpig/gin-layout/pkg/response"
)

func Resp() *r.Response {
	// 初始化response
	return r.NewResponse()
}

// Success 业务成功响应
func Success(c *gin.Context, data ...any) {
	if data != nil {
		Resp().WithDataSuccess(c, data[0])
		return
	}
	Resp().Success(c)
}

// Fail 业务失败响应
func Fail(c *gin.Context, code int, data ...any) {
	if data != nil {
		Resp().WithData(data[0]).FailCode(c, code)
		return
	}
	Resp().FailCode(c, code)
}
