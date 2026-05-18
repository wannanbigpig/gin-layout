package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
)

const defaultSessionRevokeReason = "管理员强制下线"

// ListSessions 分页查询在线会话列表。
func (s *LoginService) ListSessions(params *form.SessionList) *resources.Collection {
	query := query_builder.New().
		AddEq("login_status", model.LoginStatusSuccess).
		AddLike("username", params.Username).
		AddLike("ip", params.IP).
		AddEq("is_revoked", params.IsRevoked)
	if params.UID > 0 {
		query.AddEq("uid", params.UID)
	}
	if params.StartTime != "" {
		query.AddCondition("created_at >= ?", params.StartTime)
	}
	if params.EndTime != "" {
		query.AddCondition("created_at <= ?", params.EndTime)
	}
	condition, args := query.Build()

	loginLog := model.NewAdminLoginLogs()
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"uid",
			"username",
			"jwt_id",
			"ip",
			"os",
			"browser",
			"is_revoked",
			"revoked_reason",
			"revoked_at",
			"token_expires",
			"created_at",
		},
		OrderBy: "created_at DESC, id DESC",
	}

	transformer := resources.NewSessionTransformer()
	total, collection, err := model.ListPageE(loginLog, params.Page, params.PerPage, condition, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询在线会话列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// RevokeSession 撤销指定在线会话。
func (s *LoginService) RevokeSession(ctx context.Context, id uint, reason string) error {
	s.ensureRuntimeDeps()
	loginLog, err := s.loadRevocableSession(id)
	if err != nil {
		return err
	}

	revokeReason := strings.TrimSpace(reason)
	if revokeReason == "" {
		revokeReason = defaultSessionRevokeReason
	}

	if err := s.markTokensRevokedFn(ctx, []string{loginLog.JwtID}, model.RevokedCodeSystemForce, revokeReason); err != nil {
		log.Logger.Error("撤销在线会话数据库状态失败", zap.Error(err), zap.Uint("id", id), zap.String("jwt_id", loginLog.JwtID))
		return err
	}

	remainingTime := time.Until(loginLog.TokenExpires.Time)
	if err := s.writeTokenToBlacklistFn(loginLog.JwtID, remainingTime); err != nil {
		log.Logger.Warn("Redis 黑名单写入失败，保留数据库撤销状态作为兜底",
			zap.Error(err),
			zap.Bool("redis_unavailable", errors.Is(err, errRedisUnavailable)),
			zap.Uint("id", id),
			zap.String("jwt_id", loginLog.JwtID))
		return nil
	}
	return nil
}

func (s *LoginService) loadRevocableSession(id uint) (*model.AdminLoginLogs, error) {
	loginLog := model.NewAdminLoginLogs()
	if s.loginLogDB != nil {
		loginLog.SetDB(s.loginLogDB)
	}
	if err := loginLog.GetById(id); err != nil || loginLog.ID == 0 {
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Logger.Error("查询在线会话失败", zap.Error(err), zap.Uint("id", id))
		}
		return nil, e.NewBusinessError(e.NotFound)
	}
	if loginLog.LoginStatus != model.LoginStatusSuccess {
		return nil, e.NewBusinessError(e.InvalidParameter, "仅成功登录会话允许撤销")
	}
	if loginLog.IsRevoked != model.IsRevokedNo {
		return nil, e.NewBusinessError(e.InvalidParameter, "会话已撤销")
	}
	if loginLog.TokenExpires == nil || !loginLog.TokenExpires.Time.After(time.Now()) {
		return nil, e.NewBusinessError(e.InvalidParameter, "会话已过期")
	}
	if loginLog.JwtID == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "会话缺少 jwt_id")
	}
	return loginLog, nil
}
