package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	casbinx "github.com/wannanbigpig/gin-layout/internal/pkg/utils/casbin"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
)

// AdminAuthHandler 管理员权限验证中间件
func AdminAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 提取并验证Token
		accessToken, err := extractAccessToken(c)
		if err != nil {
			response.Fail(c, e.NotLogin, err.Error())
			c.Abort()
			return
		}

		// 验证Token并获取用户信息
		adminUser, ok := validateToken(c, accessToken)
		if !ok {
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

		// 设置用户信息到上下文
		setUserContext(c, adminUser)
		c.Next()
	}
}

// extractAccessToken 从请求头中提取访问令牌
func extractAccessToken(c *gin.Context) (string, error) {
	authorization := c.GetHeader("Authorization")
	return token.GetAccessToken(authorization)
}

// validateToken 验证Token并返回用户信息
func validateToken(c *gin.Context, accessToken string) (*model.AdminUser, bool) {
	loginService := permission.NewLoginService()
	loginService.SetCtx(c)
	adminUser, ok := loginService.CheckToken(accessToken)

	// 如果验证成功，解析 token 获取 jwt_id 并设置到上下文
	if ok {
		claims := &token.AdminCustomClaims{}
		if err := token.Parse(accessToken, claims, jwt.WithSubject(global.PcAdminSubject), jwt.WithIssuer(global.Issuer)); err == nil {
			c.Set("jwt_id", claims.ID)
		}
	}

	return adminUser, ok
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

// setUserContext 设置用户信息到上下文
func setUserContext(c *gin.Context, adminUser *model.AdminUser) {
	c.Set("uid", adminUser.ID)
	c.Set("username", adminUser.Username)
	c.Set("full_phone_number", adminUser.FullPhoneNumber)
	c.Set("nickname", adminUser.Nickname)
	c.Set("email", adminUser.Email)
}
