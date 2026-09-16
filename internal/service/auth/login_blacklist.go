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
	if err != nil {
		return err
	}

	if err := s.markTokensRevokedFn(context.Background(), []string{claims.ID}, model.RevokedCodeUserLogout, "用户主动登出（退出登录）"); err != nil {
		return err
	}

	if exp != nil {
		remainingTime := time.Until(exp.Time)
		if err := s.writeTokenToBlacklistFn(claims.ID, remainingTime); err != nil {
			log.Logger.Warn("Redis blacklist write failed after database revocation, treat logout as success",
				zap.String("jwt_id", claims.ID),
				zap.Bool("redis_unavailable", errors.Is(err, errRedisUnavailable)),
				zap.Error(err))
			s.kickSession(claims.UserID, claims.ID, SessionControlCodeKicked, "用户主动登出")
			return nil
		}
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

// PreparedUserTokenRevocation 保存已在事务中撤销的会话，供提交后同步 Redis 和在线连接。
type PreparedUserTokenRevocation struct {
	userID        uint
	revokedCode   uint8
	revokedReason string
	loginLogs     []model.AdminLoginLogs
}

// PrepareUserTokenRevocation 在调用方事务内撤销数据库会话，避免账号变更提交后仍有可用 token 的窗口。
func (s *LoginService) PrepareUserTokenRevocation(tx *gorm.DB, userID uint, revokedCode uint8, revokedReason string) (*PreparedUserTokenRevocation, error) {
	prepared := &PreparedUserTokenRevocation{
		userID:        userID,
		revokedCode:   revokedCode,
		revokedReason: revokedReason,
	}
	if userID == 0 {
		return prepared, nil
	}

	loginLog := model.NewAdminLoginLogs()
	if tx != nil {
		loginLog.SetDB(tx)
	} else if s.loginLogDB != nil {
		loginLog.SetDB(s.loginLogDB)
	} else if _, err := loginLog.GetDB(); err != nil {
		return nil, err
	}

	loginLogs, err := loginLog.FindActiveTokensByUserIdForUpdate(userID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	prepared.loginLogs = loginLogs
	loginLogIDs := collectLoginLogIDs(loginLogs)
	if len(loginLogIDs) == 0 {
		return prepared, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), revokeLogAsyncTimeout)
	defer cancel()
	if err := s.markLoginLogRowsRevoked(ctx, loginLogIDs, revokedCode, revokedReason, tx); err != nil {
		return nil, err
	}
	return prepared, nil
}

// CompleteUserTokenRevocation 在数据库事务提交后同步缓存并踢出在线连接。
func (s *LoginService) CompleteUserTokenRevocation(prepared *PreparedUserTokenRevocation) error {
	if prepared == nil || prepared.userID == 0 {
		return nil
	}
	blacklistErr := s.addTokensToBlacklist(prepared.loginLogs)
	s.kickSession(prepared.userID, "", int(prepared.revokedCode), prepared.revokedReason)
	return blacklistErr
}

// RevokeUserTokens 撤销用户所有未过期的 token。
func (s *LoginService) RevokeUserTokens(userId uint, revokedCode uint8, revokedReason string) error {
	if userId == 0 {
		return nil
	}

	loginLog := model.NewAdminLoginLogs()
	db := s.loginLogDB
	if db == nil {
		var err error
		db, err = loginLog.GetDB()
		if err != nil {
			return err
		}
	}
	var prepared *PreparedUserTokenRevocation
	if err := db.Transaction(func(tx *gorm.DB) error {
		var prepareErr error
		prepared, prepareErr = s.PrepareUserTokenRevocation(tx, userId, revokedCode, revokedReason)
		return prepareErr
	}); err != nil {
		return err
	}
	return s.CompleteUserTokenRevocation(prepared)
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
