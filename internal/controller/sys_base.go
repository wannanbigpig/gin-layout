package controller

import (
	stderrors "errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	r "github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/admin"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

// Api 控制器基类
type Api struct {
	errors.Error
}

// Success 业务成功响应
func (api Api) Success(c *gin.Context, data ...any) {
	response := r.Resp()
	if len(data) > 0 && data[0] != nil {
		response.WithDataSuccess(c, data[0])
		return
	}
	response.Success(c)
}

// FailCode 业务失败响应（使用错误码）
func (api Api) FailCode(c *gin.Context, code int, data ...any) {
	response := r.Resp()
	if len(data) > 0 && data[0] != nil {
		response.WithData(data[0]).FailCode(c, code)
		return
	}
	response.FailCode(c, code)
}

// FailCodeByKey 业务失败响应（使用错误码 + 文案 key）。
func (api Api) FailCodeByKey(c *gin.Context, code int, key string, args ...any) {
	r.Resp().FailCodeByKey(c, code, key, args...)
}

// Fail 业务失败响应（自定义错误消息）
func (api Api) Fail(c *gin.Context, code int, message string, data ...any) {
	response := r.Resp()
	if len(data) > 0 && data[0] != nil {
		response.WithData(data[0]).Fail(c, code, message)
		return
	}
	response.Fail(c, code, message)
}

// Err 统一错误处理
// 判断错误类型是自定义类型则自动返回错误中携带的code和message，否则返回服务器错误
func (api Api) Err(c *gin.Context, err error) {
	businessError, parseErr := api.AsBusinessError(err)
	if parseErr != nil {
		requestID := c.GetString(global.ContextKeyRequestID)
		if errors.IsDependencyNotReady(parseErr) || stderrors.Is(parseErr, model.ErrDBUninitialized) {
			log.Logger.Warn("Service dependency not ready",
				zap.String("requestId", requestID),
				zap.Error(parseErr))
			api.FailCode(c, errors.ServiceDependencyNotReady)
			return
		}
		log.Logger.Warn("Unknown error:", zap.String("requestId", requestID), zap.Error(parseErr))
		api.FailCode(c, errors.ServerErr)
		return
	}

	if businessError.HasExplicitMessage() {
		api.Fail(c, businessError.GetCode(), businessError.GetMessage())
		return
	}
	if businessError.HasMessageKey() {
		api.FailCodeByKey(c, businessError.GetCode(), businessError.GetMessageKey(), businessError.GetMessageArgs()...)
		return
	}
	api.FailCode(c, businessError.GetCode())
}

// GetCurrentUserID 获取当前登录用户的ID
func (api Api) GetCurrentUserID(c *gin.Context) uint {
	return c.GetUint(global.ContextKeyUID)
}

// GetCurrentAdminUserSnapshot 获取当前登录用户的 claims 快照投影，不代表数据库最新状态。
func (api Api) GetCurrentAdminUserSnapshot(c *gin.Context) *model.AdminUser {
	if principal := auth.GetAuthPrincipal(c); principal != nil {
		return principal.AdminUser()
	}
	return nil
}

// GetCurrentAdminUserDetail 获取当前登录用户的数据库最新详情。
func (api Api) GetCurrentAdminUserDetail(c *gin.Context) (*resources.AdminUserResources, error) {
	uid := api.GetCurrentUserID(c)
	if uid == 0 {
		return nil, errors.NewBusinessError(errors.NotLogin)
	}
	return admin.NewAdminUserService().GetUserInfo(uid)
}
