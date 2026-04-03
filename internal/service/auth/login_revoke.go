package auth

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"go.uber.org/zap"
)

// isTokenRevokedInLog 检查登录日志表中 token 是否被撤销。
func (s *LoginService) isTokenRevokedInLog(jwtId string) bool {
	if jwtId == "" {
		return false
	}

	loginLog := model.NewAdminLoginLogs()
	db, err := loginLog.GetDB()
	if err != nil {
		return false
	}
	err = db.Where("jwt_id = ? AND deleted_at = 0", jwtId).Select("is_revoked").First(loginLog).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Logger.Error("检查token撤销状态失败", zap.Error(err), zap.String("jwt_id", jwtId))
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
		if err := s.markTokensRevoked(ids, revokedCode, revokedReason); err != nil {
			log.Logger.Error("异步更新登录日志 token 撤销状态失败", zap.Error(err), zap.Strings("jwt_ids", ids))
		}
	}(append([]string(nil), jwtIds...))
}

func (s *LoginService) markTokensRevoked(jwtIds []string, revokedCode uint8, revokedReason string) error {
	now := time.Now()
	revokedAt := utils.FormatDate{Time: now}
	updates := map[string]interface{}{
		"is_revoked":     model.IsRevokedYes,
		"revoked_code":   revokedCode,
		"revoked_reason": revokedReason,
		"revoked_at":     revokedAt,
		"updated_at":     now,
	}

	loginLog := model.NewAdminLoginLogs()
	db, err := loginLog.GetDB(loginLog)
	if err != nil {
		return err
	}

	return db.
		Where("jwt_id IN ? AND deleted_at = 0 AND is_revoked = ?", jwtIds, model.IsRevokedNo).
		Updates(updates).Error
}
