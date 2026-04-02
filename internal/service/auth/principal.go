package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

// AuthPrincipal 表示一次请求中已验证的认证主体。
type AuthPrincipal struct {
	Claims          *token.AdminCustomClaims
	JWTID           string
	User            *model.AdminUser
	UserID          uint
	Username        string
	Nickname        string
	Email           string
	FullPhoneNumber string
}

func newAuthPrincipal(user *model.AdminUser, claims *token.AdminCustomClaims) *AuthPrincipal {
	if user == nil {
		return nil
	}

	principal := &AuthPrincipal{
		User:            user,
		UserID:          user.ID,
		Username:        user.Username,
		Nickname:        user.Nickname,
		Email:           user.Email,
		FullPhoneNumber: user.FullPhoneNumber,
	}
	if claims != nil {
		principal.Claims = claims
		principal.JWTID = claims.ID
	}
	return principal
}

// StoreAuthPrincipal 将认证主体写入上下文。
func StoreAuthPrincipal(c *gin.Context, principal *AuthPrincipal) {
	if c == nil || principal == nil {
		return
	}
	c.Set(global.ContextKeyAuthPrincipal, principal)
	c.Set(global.ContextKeyUID, principal.UserID)
}

// GetAuthPrincipal 从上下文中读取认证主体。
func GetAuthPrincipal(c *gin.Context) *AuthPrincipal {
	if c == nil {
		return nil
	}
	if value, exists := c.Get(global.ContextKeyAuthPrincipal); exists {
		if principal, ok := value.(*AuthPrincipal); ok {
			return principal
		}
	}
	return nil
}
