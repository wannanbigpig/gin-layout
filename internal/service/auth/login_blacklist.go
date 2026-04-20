package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"go.uber.org/zap"
)

const redisOpTimeout = 3 * time.Second

var errRedisUnavailable = errors.New("redis client is not available")

// Logout 退出登录。
func (s *LoginService) Logout(accessToken string) error {
	s.ensureRuntimeDeps()
	claims, err := s.parseToken(accessToken)
	if err != nil {
		return err
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return err
	}

	if err := s.markTokensRevokedFn(context.Background(), []string{claims.ID}, model.RevokedCodeUserLogout, "用户主动登出（退出登录）"); err != nil {
		return err
	}

	remainingTime := time.Until(exp.Time)
	if err := s.writeTokenToBlacklistFn(claims.ID, remainingTime); err != nil {
		log.Logger.Warn("Redis blacklist write failed after database revocation, treat logout as success",
			zap.String("jwt_id", claims.ID),
			zap.Bool("redis_unavailable", errors.Is(err, errRedisUnavailable)),
			zap.Error(err))
		return nil
	}

	return nil
}

// parseToken 解析 Token。
func (s *LoginService) parseToken(accessToken string) (*token.AdminCustomClaims, error) {
	claims := new(token.AdminCustomClaims)
	secret := []byte(s.currentConfig().Jwt.SecretKey)
	parsedToken, err := jwt.ParseWithClaims(accessToken, claims, func(jwtToken *jwt.Token) (interface{}, error) {
		if _, ok := jwtToken.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", jwtToken.Header["alg"])
		}
		return secret, nil
	}, jwt.WithSubject(global.PcAdminSubject), jwt.WithIssuer(global.Issuer))
	if err != nil {
		return nil, err
	}
	if !parsedToken.Valid {
		return nil, e.NewBusinessError(1, "invalid token")
	}
	return claims, nil
}

// IsInBlacklist 判断 Token 是否在黑名单中。
func (s *LoginService) IsInBlacklist(jwtId string) (bool, error) {
	redisClient := data.RedisClient()
	if redisClient == nil {
		if err := data.GetRedisInitError(); err != nil {
			return false, fmt.Errorf("%w: %v", errRedisUnavailable, err)
		}
		return false, errRedisUnavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()
	result, err := redisClient.Exists(ctx, s.getBlacklistKey(jwtId)).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

func (s *LoginService) writeTokenToBlacklist(jwtID string, remainingTime time.Duration) error {
	if jwtID == "" || remainingTime <= 0 {
		return nil
	}

	redisClient := data.RedisClient()
	if redisClient == nil {
		if err := data.GetRedisInitError(); err != nil {
			return fmt.Errorf("%w: %v", errRedisUnavailable, err)
		}
		return errRedisUnavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()
	return redisClient.Set(ctx, s.getBlacklistKey(jwtID), "1", remainingTime).Err()
}

// getBlacklistKey 获取 Redis 黑名单 key。
func (s *LoginService) getBlacklistKey(jwtId string) string {
	return blacklistPrefix + jwtId
}

// addTokensToBlacklist 批量将 token 加入 Redis 黑名单。
func (s *LoginService) addTokensToBlacklist(loginLogs []model.AdminLoginLogs) {
	if len(loginLogs) == 0 {
		return
	}

	redisClient := data.RedisClient()
	if redisClient == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()
	pipe := redisClient.Pipeline()

	now := time.Now()
	queued := 0
	for _, item := range loginLogs {
		if item.JwtID == "" {
			continue
		}
		remainingTime := s.calculateRemainingTime(item.TokenExpires, now)
		if remainingTime <= 0 {
			continue
		}
		blacklistKey := s.getBlacklistKey(item.JwtID)
		pipe.Set(ctx, blacklistKey, "1", remainingTime)
		queued++
	}

	if queued == 0 {
		return
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Logger.Error("批量将 token 加入 Redis 黑名单失败", zap.Error(err), zap.Int("count", queued))
	}
}

// calculateRemainingTime 计算 token 剩余过期时间。
func (s *LoginService) calculateRemainingTime(tokenExpires *utils.FormatDate, now time.Time) time.Duration {
	if tokenExpires != nil {
		remainingTime := tokenExpires.Time.Sub(now)
		if remainingTime > 0 {
			return remainingTime
		}
		return 0
	}
	return defaultTokenTTL
}

// RevokeUserTokens 撤销用户所有未过期的 token。
func (s *LoginService) RevokeUserTokens(userId uint, revokedCode uint8, revokedReason string, tx ...*gorm.DB) error {
	if userId == 0 {
		return nil
	}

	loginLog := model.NewAdminLoginLogs()
	if existingTx := access.FirstTx(tx); existingTx != nil {
		loginLog.SetDB(existingTx)
	} else {
		if _, err := loginLog.GetDB(); err != nil {
			return err
		}
	}

	loginLogs, err := loginLog.FindActiveTokensByUserId(userId, time.Now())
	if err != nil || len(loginLogs) == 0 {
		return err
	}

	jwtIds := collectJWTIDs(loginLogs)
	if len(jwtIds) == 0 {
		return nil
	}

	s.addTokensToBlacklist(loginLogs)
	s.revokeTokenInLogAsync(jwtIds, revokedCode, revokedReason)
	return nil
}

func collectJWTIDs(loginLogs []model.AdminLoginLogs) []string {
	jwtIds := make([]string, 0, len(loginLogs))
	for _, item := range loginLogs {
		if item.JwtID != "" {
			jwtIds = append(jwtIds, item.JwtID)
		}
	}
	return jwtIds
}
