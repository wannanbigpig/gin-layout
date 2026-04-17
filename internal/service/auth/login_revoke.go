package auth

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"go.uber.org/zap"
)

const revokeLogAsyncTimeout = 5 * time.Second

// isTokenRevokedInLog 检查登录日志表中 token 是否被撤销。
func (s *LoginService) isTokenRevokedInLog(jwtId string) bool {
	if jwtId == "" {
		return false
	}

	loginLog := model.NewAdminLoginLogs()
	err := loginLog.FindByJwtId(jwtId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logRevokeError("检查token撤销状态失败", zap.Error(err), zap.String("jwt_id", jwtId))
		}
		return false
	}

	return loginLog.IsRevoked == model.IsRevokedYes
}

func (s *LoginService) revokeTokenInLogAsync(jwtIds []string, revokedCode uint8, revokedReason string) {
	if len(jwtIds) == 0 {
		return
	}

	go func(ids []string) {
		defer func() {
			if r := recover(); r != nil {
				logRevokeError("异步更新 token 撤销状态 panic", zap.Any("recover", r))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), revokeLogAsyncTimeout)
		defer cancel()

		if err := s.markTokensRevoked(ctx, ids, revokedCode, revokedReason); err != nil {
			logRevokeError("异步更新登录日志 token 撤销状态失败", zap.Error(err), zap.Strings("jwt_ids", ids))
		}
	}(append([]string(nil), jwtIds...))
}

func (s *LoginService) markTokensRevoked(ctx context.Context, jwtIds []string, revokedCode uint8, revokedReason string) error {
	now := time.Now()
	revokedAt := utils.FormatDate{Time: now}

	loginLog := model.NewAdminLoginLogs()
	db, err := loginLog.GetDB()
	if err != nil {
		return err
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	loginLog.SetDB(db)
	return loginLog.UpdateRevokedStatusByJwtIds(jwtIds, revokedCode, revokedReason, revokedAt)
}

func logRevokeError(message string, fields ...zap.Field) {
	if log.Logger == nil {
		return
	}
	log.Logger.Error(message, fields...)
}
