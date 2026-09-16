package auth

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
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
		if !errors.Is(err, gorm.ErrRecordNotFound) && log.Logger != nil {
			log.Logger.Error("检查token撤销状态失败", zap.Error(err), zap.String("jwt_id", jwtId))
		}
		return false
	}

	return loginLog.IsRevoked == model.IsRevokedYes
}

func (s *LoginService) markTokensRevoked(ctx context.Context, jwtIds []string, revokedCode uint8, revokedReason string) error {
	now := time.Now().UTC()
	revokedAt := utils.FormatDate{Time: now}

	loginLog := model.NewAdminLoginLogs()
	db := s.loginLogDB
	if db == nil {
		var err error
		db, err = loginLog.GetDB()
		if err != nil {
			return err
		}
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	loginLog.SetDB(db)
	return loginLog.UpdateRevokedStatusByJwtIds(jwtIds, revokedCode, revokedReason, revokedAt)
}

func (s *LoginService) markLoginLogRowsRevoked(ctx context.Context, ids []uint, revokedCode uint8, revokedReason string, tx ...*gorm.DB) error {
	if len(ids) == 0 {
		return nil
	}
	revokedAt := utils.FormatDate{Time: time.Now().UTC()}
	loginLog := model.NewAdminLoginLogs()
	db := s.loginLogDB
	if existingTx := access.FirstTx(tx); existingTx != nil {
		db = existingTx
	}
	if db == nil {
		var err error
		db, err = loginLog.GetDB()
		if err != nil {
			return err
		}
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	loginLog.SetDB(db)
	return loginLog.UpdateRevokedStatusByIds(ids, revokedCode, revokedReason, revokedAt)
}
