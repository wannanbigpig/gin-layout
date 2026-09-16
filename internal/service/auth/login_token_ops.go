package auth

import (
	"errors"
	"time"

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
	return principal, true
}

// isPrincipalValid 检查 token 对应主体是否仍然有效。
func (s *LoginService) isPrincipalValid(claims *token.AdminCustomClaims) bool {
	s.ensureRuntimeDeps()
	if claims == nil || claims.ID == "" {
		return false
	}
	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil || !expiresAt.Time.After(time.Now()) {
		return false
	}
	inBlacklist, err := s.blacklistLookupFn(claims.ID)
	if err == nil {
		if inBlacklist {
			return false
		}
		if s.mysqlReadyFn() {
			return s.isTokenSessionValid(claims.ID)
		}
		// 登录会话记录是 token 状态的持久化真相。即使 Redis 黑名单未命中，
		// MySQL 不可用时也无法确认会话仍存在且未撤销，必须拒绝认证。
		return false
	}

	if !s.mysqlReadyFn() {
		return false
	}

	if log.Logger != nil && s.shouldLogRedisFallback(err) {
		log.Logger.Warn("Redis 黑名单查询失败，回退到数据库撤销状态校验", zap.Error(err), zap.String("jwt_id", claims.ID))
	}
	return s.isTokenSessionValid(claims.ID)
}

func (s *LoginService) isTokenSessionValid(jwtID string) bool {
	revoked, err := s.tokenRevokedLookupFn(jwtID)
	if err != nil {
		if log.Logger != nil {
			log.Logger.Error("检查token撤销状态失败，拒绝认证", zap.Error(err), zap.String("jwt_id", jwtID))
		}
		return false
	}
	return !revoked
}

// IsPrincipalValid 检查 Token 对应主体是否仍然有效，用于 WebSocket 等长连接外部反查。
func (s *LoginService) IsPrincipalValid(claims *token.AdminCustomClaims) bool {
	return s.isPrincipalValid(claims)
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
