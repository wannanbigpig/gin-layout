package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	accesssvc "github.com/wannanbigpig/gin-layout/internal/service/access"
)

// AdminAuthHandler 依赖 ParseTokenHandler 预先写入用户上下文。
func AdminAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.GetUint(global.ContextKeyUID)
		if uid == 0 {
			response.Fail(c, e.NotLogin, "请先登录")
			c.Abort()
			return
		}

		adminUser := getUserFromContext(c)
		if adminUser == nil {
			response.Fail(c, e.NotLogin, "登录已失效，请重新登录")
			c.Abort()
			return
		}

		if !isSuperAdmin(adminUser) {
			if err := checkPermission(c, adminUser); err != nil {
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
func isSuperAdmin(adminUser *model.AdminUser) bool {
	return adminUser.IsSuperAdmin == global.Yes || adminUser.ID == global.SuperAdminId
}

// checkPermission 检查接口权限
func checkPermission(c *gin.Context, adminUser *model.AdminUser) error {
	enforcer, err := casbinx.GetEnforcer()
	if err != nil {
		log.Logger.Error("权限验证初始化失败", zap.Error(err))
		return e.NewBusinessError(e.ServerErr, "权限验证初始化失败")
	}

	userKey := fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, adminUser.ID)
	path := c.Request.URL.Path
	method := c.Request.Method

	ok, err := enforcer.Enforce(userKey, path, method)
	if err != nil {
		log.Logger.Error("权限验证失败", zap.Error(err))
		return e.NewBusinessError(e.ServerErr, "权限验证失败")
	}

	if !ok {
		if accesssvc.NewApiRouteCacheService().CheckoutRouteIsAuth(path, method) {
			return e.NewBusinessError(e.AuthorizationErr, "暂无接口操作权限")
		}
	}

	return nil
}

// getUserFromContext 优先复用上下文中的用户对象，缺失时再回源数据库。
func getUserFromContext(c *gin.Context) *model.AdminUser {
	if user, exists := c.Get(global.ContextKeyAdminUser); exists {
		if adminUser, ok := user.(*model.AdminUser); ok {
			return adminUser
		}
	}

	uid := c.GetUint(global.ContextKeyUID)
	if uid == 0 {
		return nil
	}

	adminUser := model.NewAdminUsers()
	if err := adminUser.GetById(uid); err != nil {
		return nil
	}

	setUserContext(c, adminUser, "")
	return adminUser
}
