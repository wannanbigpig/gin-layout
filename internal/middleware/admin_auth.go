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
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

type permissionDeps struct {
	// loadEnforcer 加载 Casbin enforcer。
	loadEnforcer func() (*casbinx.CasbinEnforcer, error)
}

func newDefaultPermissionDeps() permissionDeps {
	return permissionDeps{
		loadEnforcer: casbinx.GetEnforcer,
	}
}

// AdminLoginHandler 校验管理员登录态，依赖 ParseTokenHandler 预先写入用户上下文。
func AdminLoginHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if requireAdminLogin(c) == nil {
			return
		}
		c.Next()
	}
}

// AdminPermissionHandler 校验管理员接口权限，应挂载在 AdminLoginHandler 之后。
func AdminPermissionHandler() gin.HandlerFunc {
	return AdminPermissionHandlerWithDeps(newDefaultPermissionDeps())
}

// AdminPermissionHandlerWithDeps 支持注入依赖，便于测试隔离。
func AdminPermissionHandlerWithDeps(deps permissionDeps) gin.HandlerFunc {
	if deps.loadEnforcer == nil {
		deps.loadEnforcer = casbinx.GetEnforcer
	}
	return func(c *gin.Context) {
		principal := requireAdminLogin(c)
		if principal == nil {
			return
		}

		if !isSuperAdmin(principal) {
			if err := checkPermission(c, principal.UserID, deps); err != nil {
				if businessErr, ok := err.(*e.BusinessError); ok {
					if businessErr.HasMessageKey() {
						response.FailCodeByKey(c, businessErr.GetCode(), businessErr.GetMessageKey(), businessErr.GetMessageArgs()...)
					} else {
						response.FailCode(c, businessErr.GetCode())
					}
				} else {
					response.FailCode(c, e.ServerErr)
				}
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func requireAdminLogin(c *gin.Context) *auth.AuthPrincipal {
	if c.GetUint(global.ContextKeyUID) == 0 {
		response.FailCode(c, e.NotLogin)
		c.Abort()
		return nil
	}

	principal := auth.GetAuthPrincipal(c)
	if principal == nil {
		response.FailCodeByKey(c, e.NotLogin, e.MsgKeyAuthSessionExpired)
		c.Abort()
		return nil
	}
	return principal
}

// isSuperAdmin 判断是否为超级管理员
func isSuperAdmin(principal *auth.AuthPrincipal) bool {
	return principal != nil && (principal.IsSuperAdmin == global.Yes || principal.UserID == global.SuperAdminId)
}

// checkPermission 检查接口权限
func checkPermission(c *gin.Context, userID uint, deps permissionDeps) error {
	enforcer, err := loadEnforcer(deps)
	if err != nil {
		return err
	}
	return enforcePermission(enforcer, c, userID)
}

func loadEnforcer(deps permissionDeps) (*casbinx.CasbinEnforcer, error) {
	enforcer, err := deps.loadEnforcer()
	if err != nil {
		log.Logger.Error("权限验证初始化失败", zap.Error(err))
		return nil, e.NewBusinessErrorWithKey(e.ServerErr, e.MsgKeyAuthPermissionInitFailed)
	}
	return enforcer, nil
}

func enforcePermission(enforcer *casbinx.CasbinEnforcer, c *gin.Context, userID uint) error {
	userKey := fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, userID)
	path := c.Request.URL.Path
	method := c.Request.Method

	ok, err := enforcer.Enforce(userKey, path, method)
	if err != nil {
		log.Logger.Error("权限验证失败", zap.Error(err))
		return e.NewBusinessErrorWithKey(e.ServerErr, e.MsgKeyAuthPermissionCheckFailed)
	}

	if !ok {
		return e.NewBusinessErrorWithKey(e.AuthorizationErr, e.MsgKeyAuthAPIOperationDenied)
	}

	return nil
}
