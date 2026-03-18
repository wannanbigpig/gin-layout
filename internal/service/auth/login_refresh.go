package auth

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mssola/useragent"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

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

// buildRefreshLogInfo 构建刷新 token 日志信息。
func (s *LoginService) buildRefreshLogInfo(c *gin.Context) LoginLogInfo {
	return s.BuildLoginLogInfo(c)
}

// tryRefreshToken 尝试自动刷新 Token。
func (s *LoginService) tryRefreshToken(claims *token.AdminCustomClaims) {
	if !s.shouldRefreshToken(claims) {
		return
	}

	lockKey := s.buildRefreshLockKey(claims)
	unlock := s.acquireRefreshLock(lockKey, claims)
	if unlock == nil {
		return
	}
	defer unlock()

	s.doRefreshToken(claims)
}

// shouldRefreshToken 判断是否需要刷新 token。
func (s *LoginService) shouldRefreshToken(claims *token.AdminCustomClaims) bool {
	cfg := config.GetConfig()
	if cfg.Jwt.RefreshTTL <= 0 || s.GetCtx() == nil {
		return false
	}

	exp, err := claims.GetExpirationTime()
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
	if config.GetConfig().Redis.Enable && data.RedisClient() != nil {
		return s.acquireRedisLock(lockKey, claims)
	}
	return s.acquireMemoryLock(lockKey)
}

// acquireRedisLock 获取 Redis 分布式锁。
func (s *LoginService) acquireRedisLock(lockKey string, claims *token.AdminCustomClaims) func() {
	ctx := context.Background()
	redisClient := data.RedisClient()
	locked, err := redisClient.SetNX(ctx, lockKey, "1", refreshLockTTL).Result()
	if err != nil {
		log.Logger.Warn("获取刷新token Redis锁失败，降级到内存锁", zap.Error(err), zap.Uint("user_id", claims.UserID), zap.String("jwt_id", claims.ID))
		return s.acquireMemoryLock(lockKey)
	}
	if !locked {
		return nil
	}
	return func() {
		_ = redisClient.Del(ctx, lockKey).Err()
	}
}

// acquireMemoryLock 获取内存锁。
func (s *LoginService) acquireMemoryLock(lockKey string) func() {
	memLock := memoryRefreshLock.getLock(lockKey)
	memLock.Lock()
	return memLock.Unlock
}

// doRefreshToken 执行刷新 token。
func (s *LoginService) doRefreshToken(claims *token.AdminCustomClaims) {
	logInfo := s.buildRefreshLogInfo(s.GetCtx())
	tokenResponse, err := s.Refresh(claims.UserID, logInfo)
	if err != nil {
		log.Logger.Warn("自动刷新token失败", zap.Error(err), zap.Uint("user_id", claims.UserID), zap.String("jwt_id", claims.ID))
		return
	}
	if tokenResponse == nil {
		return
	}

	ctx := s.GetCtx()
	ctx.Writer.Header().Set("refresh-access-token", tokenResponse.AccessToken)
	ctx.Writer.Header().Set("refresh-exp", strconv.FormatInt(tokenResponse.ExpiresAt, 10))
}
