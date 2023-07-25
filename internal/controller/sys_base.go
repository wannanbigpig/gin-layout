package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	r "github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"go.uber.org/zap"
)

type Api struct {
	errors.Error
}

// Success 业务成功响应
func (api Api) Success(c *gin.Context, data ...any) {
	response := r.Resp()
	if data != nil {
		response.WithDataSuccess(c, data[0])
		return
	}
	response.Success(c)
}

// FailCode 业务失败响应
func (api Api) FailCode(c *gin.Context, code int, data ...any) {
	response := r.Resp()
	if data != nil {
		response.WithData(data[0]).FailCode(c, code)
		return
	}
	response.FailCode(c, code)
}

// Fail 业务失败响应
func (api Api) Fail(c *gin.Context, code int, message string, data ...any) {
	response := r.Resp()
	if data != nil {
		response.WithData(data[0]).FailCode(c, code, message)
		return
	}
	response.FailCode(c, code, message)
}

// Err 判断错误类型是自定义类型则自动返回错误中携带的code和message，否则返回服务器错误
func (api Api) Err(c *gin.Context, e error) {
	businessError, err := api.AsBusinessError(e)
	if err != nil {
		log.Logger.Warn("Unknown error:", zap.Any("Error reason:", err))
		api.FailCode(c, errors.ServerError)
		return
	}

	api.Fail(c, businessError.GetCode(), businessError.GetMessage())
}
