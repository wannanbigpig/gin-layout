package auth

import (
	"errors"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"go.uber.org/zap"
)

var blacklistLookup = func(s *LoginService, jwtID string) (bool, error) {
	return s.IsInBlacklist(jwtID)
}

var tokenRevokedLookup = func(s *LoginService, jwtID string) bool {
	return s.isTokenRevokedInLog(jwtID)
}

var mysqlReadyLookup = data.MysqlReady

var tryRefreshPrincipal = func(s *LoginService, principal *AuthPrincipal) {
	s.tryRefreshToken(principal)
}

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
	if mysqlReadyLookup() {
		tryRefreshPrincipal(s, principal)
	}
	return principal, true
}

// isPrincipalValid 检查 token 对应主体是否仍然有效。
func (s *LoginService) isPrincipalValid(claims *token.AdminCustomClaims) bool {
	if claims == nil {
		return false
	}
	inBlacklist, err := blacklistLookup(s, claims.ID)
	if err == nil {
		return !inBlacklist
	}

	if !mysqlReadyLookup() {
		return false
	}

	if log.Logger != nil && shouldLogRedisFallback(err) {
		log.Logger.Warn("Redis 黑名单查询失败，回退到数据库撤销状态校验", zap.Error(err), zap.String("jwt_id", claims.ID))
	}
	return !tokenRevokedLookup(s, claims.ID)
}

func shouldLogRedisFallback(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errRedisUnavailable) {
		cfg := config.GetConfig()
		return cfg != nil && cfg.Redis.Enable
	}
	return true
}
