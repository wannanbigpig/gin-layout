package auth

import (
	"errors"

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
	return principal.AdminUser(), true
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
	s.ensureRuntimeDeps()
	if claims == nil {
		return nil, false
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return nil, false
	}

	if !s.isPrincipalValid(claims) {
		return nil, false
	}

	principal := newAuthPrincipalFromClaims(claims)
	if s.mysqlReadyFn() {
		s.tryRefreshPrincipalFn(principal)
	}
	return principal, true
}

// isPrincipalValid 检查 token 对应主体是否仍然有效。
func (s *LoginService) isPrincipalValid(claims *token.AdminCustomClaims) bool {
	s.ensureRuntimeDeps()
	if claims == nil {
		return false
	}
	inBlacklist, err := s.blacklistLookupFn(claims.ID)
	if err == nil {
		return !inBlacklist
	}

	if !s.mysqlReadyFn() {
		return false
	}

	if log.Logger != nil && s.shouldLogRedisFallback(err) {
		log.Logger.Warn("Redis 黑名单查询失败，回退到数据库撤销状态校验", zap.Error(err), zap.String("jwt_id", claims.ID))
	}
	return !s.tokenRevokedLookupFn(claims.ID)
}

func (s *LoginService) shouldLogRedisFallback(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errRedisUnavailable) {
		cfg := s.currentConfig()
		return cfg != nil && cfg.Redis.Enable
	}
	return true
}
