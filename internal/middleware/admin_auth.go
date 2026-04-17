package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	"github.com/wannanbigpig/gin-layout/internal/global"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	accesssvc "github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

var apiRouteCache = accesssvc.NewApiRouteCacheService()

// AdminAuthHandler 依赖 ParseTokenHandler 预先写入用户上下文。
func AdminAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.GetUint(global.ContextKeyUID)
		if uid == 0 {
			response.Fail(c, e.NotLogin, "请先登录")
			c.Abort()
			return
		}

		principal := auth.GetAuthPrincipal(c)
		if principal == nil {
			response.Fail(c, e.NotLogin, "登录已失效，请重新登录")
			c.Abort()
			return
		}

		if !isSuperAdmin(principal) {
			if err := checkPermission(c, principal.UserID); err != nil {
				if businessErr, ok := err.(*e.BusinessError); ok {
					response.Fail(c, businessErr.GetCode(), businessErr.GetMessage())
				} else {
					response.Fail(c, e.ServerErr, "权限验证失败")
				}
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// isSuperAdmin 判断是否为超级管理员
func isSuperAdmin(principal *auth.AuthPrincipal) bool {
	return principal != nil && (principal.IsSuperAdmin == global.Yes || principal.UserID == global.SuperAdminId)
}

// checkPermission 检查接口权限
func checkPermission(c *gin.Context, userID uint) error {
	enforcer, err := casbinx.GetEnforcer()
	if err != nil {
		log.Logger.Error("权限验证初始化失败", zap.Error(err))
		return e.NewBusinessError(e.ServerErr, "权限验证初始化失败")
	}

	userKey := fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, userID)
	path := c.Request.URL.Path
	method := c.Request.Method

	ok, err := enforcer.Enforce(userKey, path, method)
	if err != nil {
		log.Logger.Error("权限验证失败", zap.Error(err))
		return e.NewBusinessError(e.ServerErr, "权限验证失败")
	}

	if !ok {
		if apiRouteCache.CheckoutRouteIsAuth(path, method) {
			return e.NewBusinessError(e.AuthorizationErr, "暂无接口操作权限")
		}
	}

	return nil
}
