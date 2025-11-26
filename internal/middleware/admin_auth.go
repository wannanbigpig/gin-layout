package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	casbinx "github.com/wannanbigpig/gin-layout/internal/pkg/utils/casbin"
)

// AdminAuthHandler 管理员权限验证中间件
// 注意：此中间件需要在ParseTokenHandler之后使用，因为需要从context获取用户信息
func AdminAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从context获取用户信息（由ParseTokenHandler设置）
		uid := c.GetUint("uid")
		if uid == 0 {
			response.Fail(c, e.NotLogin, "请先登录")
			c.Abort()
			return
		}

		// 获取用户信息（从context获取，如果不存在则查询数据库）
		adminUser := getUserFromContext(c)
		if adminUser == nil {
			response.Fail(c, e.NotLogin, "登录已失效，请重新登录")
			c.Abort()
			return
		}

		// 权限验证（非超级管理员需要检查接口权限）
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
	enforcer := casbinx.GetEnforcer()
	if enforcer.Error() != nil {
		log.Logger.Error("权限验证初始化失败", zap.Error(enforcer.Error()))
		return e.NewBusinessError(e.ServerErr, "权限验证初始化失败")
	}

	// 构建权限检查的key
	userKey := fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, adminUser.ID)
	path := c.Request.URL.Path
	method := c.Request.Method

	// 检查权限
	ok, err := enforcer.Enforce(userKey, path, method)
	if err != nil {
		log.Logger.Error("权限验证失败", zap.Error(err))
		return e.NewBusinessError(e.ServerErr, "权限验证失败")
	}

	// 如果没有权限，检查接口是否需要授权
	if !ok {
		if model.NewApi().CheckoutRouteIsAuth(path, method) {
			return e.NewBusinessError(e.AuthorizationErr, "暂无接口操作权限")
		}
	}

	return nil
}

// getUserFromContext 从context获取用户信息
func getUserFromContext(c *gin.Context) *model.AdminUser {
	// 优先从context获取完整的用户对象
	if user, exists := c.Get("admin_user"); exists {
		if adminUser, ok := user.(*model.AdminUser); ok {
			return adminUser
		}
	}

	// 如果context中没有完整对象，但有uid，则查询数据库
	uid := c.GetUint("uid")
	if uid == 0 {
		return nil
	}

	adminUser := model.NewAdminUsers()
	if err := adminUser.GetById(adminUser, uid); err != nil {
		return nil
	}

	// 将查询到的用户信息设置回context，避免重复查询
	setUserContext(c, adminUser)
	return adminUser
}
