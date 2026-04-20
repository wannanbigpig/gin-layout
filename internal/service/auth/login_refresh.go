package auth

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mssola/useragent"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/data"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

const refreshLockRedisTimeout = 2 * time.Second

// BuildLoginLogInfo 从请求上下文构建登录日志信息。
func (s *LoginService) BuildLoginLogInfo(c *gin.Context) LoginLogInfo {
	userAgentStr := c.Request.UserAgent()
	ua := useragent.New(userAgentStr)
	os := ua.OS()
	browser, _ := ua.Browser()

	return LoginLogInfo{
		IP:        c.ClientIP(),
		UserAgent: userAgentStr,
		OS:        os,
		Browser:   browser,
	}
}

// tryRefreshToken 尝试自动刷新 Token。
func (s *LoginService) tryRefreshToken(principal *AuthPrincipal) {
	s.ensureRuntimeDeps()
	if !s.mysqlReadyFn() {
		return
	}
	if !s.shouldRefreshToken(principal) {
		return
	}

	lockKey := s.buildRefreshLockKey(principal.Claims)
	unlock := s.acquireRefreshLock(lockKey, principal.Claims)
	if unlock == nil {
		return
	}
	defer unlock()

	s.doRefreshToken(principal)
}

// shouldRefreshToken 判断是否需要刷新 token。
func (s *LoginService) shouldRefreshToken(principal *AuthPrincipal) bool {
	cfg := s.currentConfig()
	if cfg.Jwt.RefreshTTL <= 0 || s.GetCtx() == nil || principal == nil || principal.Claims == nil {
		return false
	}

	exp, err := principal.Claims.GetExpirationTime()
	if err != nil || exp == nil {
		return false
	}

	refreshTTL := cfg.Jwt.RefreshTTL * time.Second
	return exp.Time.Sub(time.Now()) < refreshTTL
}

// buildRefreshLockKey 构建刷新锁 key。
func (s *LoginService) buildRefreshLockKey(claims *token.AdminCustomClaims) string {
	return refreshLockPrefix + strconv.FormatUint(uint64(claims.UserID), 10) + ":" + claims.ID
}

// acquireRefreshLock 获取刷新锁。
func (s *LoginService) acquireRefreshLock(lockKey string, claims *token.AdminCustomClaims) func() {
	cfg := s.currentConfig()
	if !(cfg.Redis.Enable && data.RedisClient() != nil) {
		return s.acquireMemoryLock(lockKey)
	}

	unlock, locked, err := s.acquireRedisLock(lockKey)
	if err != nil {
		log.Logger.Warn("获取刷新token Redis锁失败，降级到内存锁", zap.Error(err), zap.Uint("user_id", claims.UserID), zap.String("jwt_id", claims.ID))
		return s.acquireMemoryLock(lockKey)
	}
	if !locked {
		return nil
	}
	return unlock
}

// acquireRedisLock 获取 Redis 分布式锁。
func (s *LoginService) acquireRedisLock(lockKey string) (func(), bool, error) {
	redisClient := data.RedisClient()
	lockCtx, lockCancel := context.WithTimeout(context.Background(), refreshLockRedisTimeout)
	defer lockCancel()

	locked, err := redisClient.SetNX(lockCtx, lockKey, "1", refreshLockTTL).Result()
	if err != nil {
		return nil, false, err
	}
	if !locked {
		return nil, false, nil
	}
	return func() {
		unlockCtx, unlockCancel := context.WithTimeout(context.Background(), refreshLockRedisTimeout)
		defer unlockCancel()
		if err := redisClient.Del(unlockCtx, lockKey).Err(); err != nil {
			log.Logger.Warn("释放刷新 token Redis 锁失败", zap.Error(err), zap.String("lock_key", lockKey))
		}
	}, true, nil
}

// acquireMemoryLock 获取内存锁。
func (s *LoginService) acquireMemoryLock(lockKey string) func() {
	s.ensureRuntimeDeps()
	memLock := s.refreshLockStore.getLock(lockKey)
	memLock.Lock()
	return memLock.Unlock
}

// doRefreshToken 执行刷新 token。
func (s *LoginService) doRefreshToken(principal *AuthPrincipal) {
	if principal == nil || principal.Claims == nil {
		return
	}
	logInfo := s.BuildLoginLogInfo(s.GetCtx())
	tokenResponse, err := s.Refresh(principal.UserID, logInfo)
	if err != nil {
		log.Logger.Warn("自动刷新token失败", zap.Error(err), zap.Uint("user_id", principal.UserID), zap.String("jwt_id", principal.JWTID))
		return
	}
	if tokenResponse == nil {
		return
	}

	ctx := s.GetCtx()
	ctx.Writer.Header().Set("refresh-access-token", tokenResponse.AccessToken)
	ctx.Writer.Header().Set("refresh-exp", strconv.FormatInt(tokenResponse.ExpiresAt, 10))
}
