package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
)

// ParseTokenHandler 全局token解析中间件（所有路由都走）
// 功能：
//   - 尝试从请求头提取token（不强制要求）
//   - 如果token存在且有效，解析并设置用户信息到context
//   - 如果token不存在或无效，静默继续执行（用于可选认证的路由）
//
// 注意：此中间件不会阻止请求，即使token无效也会继续执行
func ParseTokenHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 提前返回：如果没有token，直接继续执行
		accessToken, err := extractAccessToken(c)
		if err != nil || accessToken == "" {
			c.Next()
			return
		}

		// 验证token并获取用户信息
		adminUser, jwtID := validateToken(c, accessToken)
		if adminUser == nil {
			// token无效，静默继续（可选认证）
			c.Next()
			return
		}

		// token有效，设置用户信息和jwt_id到上下文
		setUserContext(c, adminUser, jwtID)
		c.Next()
	}
}

// extractAccessToken 从请求头中提取访问令牌
// 返回：token字符串和错误信息（如果token不存在或格式错误）
func extractAccessToken(c *gin.Context) (string, error) {
	authorization := c.GetHeader("Authorization")
	return token.GetAccessToken(authorization)
}

// validateToken 验证Token并返回用户信息和JWT ID
// 优化：先解析token获取claims（包括jwt_id），再验证token有效性
// 返回：用户信息对象和JWT ID（如果验证成功），否则返回nil和空字符串
func validateToken(c *gin.Context, accessToken string) (*model.AdminUser, string) {
	// 先解析token获取claims（包括jwt_id），避免后续重复解析
	claims := &token.AdminCustomClaims{}
	if err := token.Parse(accessToken, claims, jwt.WithSubject(global.PcAdminSubject), jwt.WithIssuer(global.Issuer)); err != nil {
		return nil, ""
	}

	// 使用CheckToken进行完整验证（包括过期检查、用户状态、黑名单等）
	loginService := permission.NewLoginService()
	loginService.SetCtx(c)
	adminUser, ok := loginService.CheckToken(accessToken)
	if !ok {
		return nil, ""
	}

	// 验证成功，返回用户信息和jwt_id
	return adminUser, claims.ID
}

// setUserContext 设置用户信息到上下文
// 将用户的基本信息和完整对象都存储到context，供后续中间件和控制器使用
// 参数：
//   - c: gin上下文
//   - adminUser: 管理员用户对象
//   - jwtID: JWT唯一标识（用于token撤销等操作）
func setUserContext(c *gin.Context, adminUser *model.AdminUser, jwtID string) {
	// 设置用户基本信息（供日志、权限验证等使用）
	c.Set("uid", adminUser.ID)
	c.Set("username", adminUser.Username)
	c.Set("full_phone_number", adminUser.FullPhoneNumber)
	c.Set("nickname", adminUser.Nickname)
	c.Set("email", adminUser.Email)

	// 设置JWT ID（用于token撤销、黑名单等操作）
	if jwtID != "" {
		c.Set("jwt_id", jwtID)
	}

	// 将完整的用户对象也存储到context，避免后续中间件重复查询数据库
	c.Set("admin_user", adminUser)
}
