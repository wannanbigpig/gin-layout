package auth

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"go.uber.org/zap"
)

// CheckToken 检查 Token 是否有效。
func (s *LoginService) CheckToken(accessToken string) (*model.AdminUser, bool) {
	principal, ok := s.ResolvePrincipal(accessToken)
	if !ok || principal == nil {
		return nil, false
	}
	return principal.User, true
}

// ResolvePrincipal 解析并验证当前访问令牌对应的认证主体。
func (s *LoginService) ResolvePrincipal(accessToken string) (*AuthPrincipal, bool) {
	claims, err := s.parseToken(accessToken)
	if err != nil {
		return nil, false
	}
	return s.resolvePrincipalFromClaims(claims)
}

func (s *LoginService) resolvePrincipalFromClaims(claims *token.AdminCustomClaims) (*AuthPrincipal, bool) {
	if !s.isTokenValid(claims) {
		return nil, false
	}

	adminUser, err := s.getUserById(claims.UserID)
	if err != nil {
		return nil, false
	}
	if !s.isUserValid(adminUser, claims.ID) {
		return nil, false
	}

	principal := newAuthPrincipal(adminUser, claims)
	s.tryRefreshToken(principal)
	return principal, true
}

// isTokenValid 检查 Token 是否未过期。
func (s *LoginService) isTokenValid(claims *token.AdminCustomClaims) bool {
	exp, err := claims.GetExpirationTime()
	return err == nil && exp != nil
}

// isUserValid 检查用户是否有效。
func (s *LoginService) isUserValid(adminUser *model.AdminUser, jwtId string) bool {
	if adminUser.Status != model.AdminUserStatusEnabled {
		return false
	}
	inBlacklist, err := s.IsInBlacklist(jwtId)
	if err == nil {
		return !inBlacklist
	}

	log.Logger.Warn("Redis 黑名单查询失败，回退到数据库撤销状态校验", zap.Error(err), zap.String("jwt_id", jwtId))
	return !s.isTokenRevokedInLog(jwtId)
}
