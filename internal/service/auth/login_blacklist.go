package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	if strings.TrimSpace(accessToken) == "" {
		return nil
	}
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
		s.kickSession(claims.UserID, claims.ID, SessionControlCodeKicked, "用户主动登出")
		return nil
	}

	s.kickSession(claims.UserID, claims.ID, SessionControlCodeKicked, "用户主动登出")

	return nil
}

func (s *LoginService) RevokeRefreshToken(refreshToken string, revokedCode uint8, revokedReason string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	hash := s.calculateTokenHash(refreshToken)
	loginLog := model.NewAdminLoginLogs()
	db := s.loginLogDB
	if db == nil {
		var err error
		db, err = loginLog.GetDB()
		if err != nil {
			return err
		}
	}
	loginLog.SetDB(db)
	_, err := loginLog.RevokeActiveByRefreshTokenHash(hash, revokedCode, revokedReason, utils.FormatDate{Time: time.Now().UTC()})
	return err
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
		return nil, e.NewBusinessError(e.NotLogin)
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
func (s *LoginService) addTokensToBlacklist(loginLogs []model.AdminLoginLogs) error {
	if len(loginLogs) == 0 {
		return nil
	}

	now := time.Now().UTC()
	type blacklistItem struct {
		key string
		ttl time.Duration
	}
	items := make([]blacklistItem, 0, len(loginLogs))
	for _, item := range loginLogs {
		if item.JwtID == "" {
			continue
		}
		remainingTime := s.calculateRemainingTime(item.TokenExpires, now)
		if remainingTime <= 0 {
			continue
		}
		items = append(items, blacklistItem{
			key: s.getBlacklistKey(item.JwtID),
			ttl: remainingTime,
		})
	}

	if len(items) == 0 {
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
	pipe := redisClient.Pipeline()
	for _, item := range items {
		pipe.Set(ctx, item.key, "1", item.ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Logger.Error("批量将 token 加入 Redis 黑名单失败", zap.Error(err), zap.Int("count", len(items)))
		return err
	}
	return nil
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
	} else if s.loginLogDB != nil {
		loginLog.SetDB(s.loginLogDB)
	} else {
		if _, err := loginLog.GetDB(); err != nil {
			return err
		}
	}

	loginLogs, err := loginLog.FindActiveTokensByUserId(userId, time.Now().UTC())
	if err != nil || len(loginLogs) == 0 {
		return err
	}

	loginLogIds := collectLoginLogIDs(loginLogs)
	if len(loginLogIds) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), revokeLogAsyncTimeout)
	defer cancel()
	// 用户禁用/强制下线需要撤销“会话行”，因为 access token 可能已过期，
	// 但同一行上的 refresh token 仍可能有效；只按 jwt_id 更新会漏掉这类会话。
	if err := s.markLoginLogRowsRevoked(ctx, loginLogIds, revokedCode, revokedReason, tx...); err != nil {
		return err
	}
	blacklistErr := s.addTokensToBlacklist(loginLogs)
	// 全端强踢：SessionID 留空，踢掉该用户全部在线连接。
	s.kickSession(userId, "", int(revokedCode), revokedReason)
	return blacklistErr
}

// kickSession 派发单会话强踢指令（SessionID 非空仅踢该会话）。
func (s *LoginService) kickSession(userID uint, sessionID string, code int, reason string) {
	publishSessionControl(context.Background(), SessionControl{
		UserID:    userID,
		SessionID: sessionID,
		Code:      code,
		Reason:    reason,
	})
}

func collectLoginLogIDs(loginLogs []model.AdminLoginLogs) []uint {
	ids := make([]uint, 0, len(loginLogs))
	for _, item := range loginLogs {
		if item.ID != 0 {
			ids = append(ids, item.ID)
		}
	}
	return ids
}
