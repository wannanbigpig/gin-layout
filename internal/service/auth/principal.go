package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

// AuthPrincipal 表示一次请求中已验证的认证主体。
//
// 这里固定采用 claims-first 语义：中间件只保存 JWT claims 中已有的字段快照，
// 不在请求上下文里缓存完整的 AdminUser 模型，避免每个请求默认回表。
type AuthPrincipal struct {
	Claims          *token.AdminCustomClaims
	JWTID           string
	UserID          uint
	Username        string
	Nickname        string
	Email           string
	FullPhoneNumber string
	PhoneNumber     string
	CountryCode     string
	IsSuperAdmin    uint8
}

func newAuthPrincipalFromClaims(claims *token.AdminCustomClaims) *AuthPrincipal {
	if claims == nil {
		return nil
	}

	principal := &AuthPrincipal{
		UserID:          claims.UserID,
		Username:        claims.Username,
		Nickname:        claims.Nickname,
		Email:           claims.Email,
		FullPhoneNumber: claims.FullPhoneNumber,
		PhoneNumber:     claims.PhoneNumber,
		CountryCode:     claims.CountryCode,
		IsSuperAdmin:    claims.IsSuperAdmin,
		Claims:          claims,
		JWTID:           claims.ID,
	}
	return principal
}

// AdminUser 将认证主体转换为兼容旧逻辑的轻量用户对象。
// 返回值只包含 claims 中已有字段，不保证数据库实时状态。
func (p *AuthPrincipal) AdminUser() *model.AdminUser {
	if p == nil {
		return nil
	}
	return &model.AdminUser{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			BaseModel: model.BaseModel{ID: p.UserID},
		},
		Username:        p.Username,
		Nickname:        p.Nickname,
		Email:           p.Email,
		FullPhoneNumber: p.FullPhoneNumber,
		PhoneNumber:     p.PhoneNumber,
		CountryCode:     p.CountryCode,
		IsSuperAdmin:    p.IsSuperAdmin,
	}
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
