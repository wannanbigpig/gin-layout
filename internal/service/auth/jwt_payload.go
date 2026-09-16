package auth

import (
	"github.com/golang-jwt/jwt/v5"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

// adminAccessTokenClaims 是实际写入 JWT payload 的最小管理员上下文。
type adminAccessTokenClaims struct {
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	IsSuperAdmin uint8  `json:"is_super_admin"`
	jwt.RegisteredClaims
}

func (s *LoginService) generateAccessToken(claims token.AdminCustomClaims) (string, error) {
	return token.Generate(adminAccessTokenClaims{
		UserID:           claims.UserID,
		Username:         claims.Username,
		IsSuperAdmin:     claims.IsSuperAdmin,
		RegisteredClaims: claims.RegisteredClaims,
	})
}
