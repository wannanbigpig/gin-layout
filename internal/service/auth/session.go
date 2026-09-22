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
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
)

const defaultSessionRevokeReason = "管理员强制下线"

// ListSessions 分页查询在线会话列表。
func (s *LoginService) ListSessions(params *form.SessionList) (*resources.Collection, error) {
	query := query_builder.New().
		AddEq("login_status", model.LoginStatusSuccess).
		AddLike("username", params.Username).
		AddLike("ip", params.IP).
		AddEq("is_revoked", params.IsRevoked)
	if params.UID > 0 {
		query.AddEq("uid", params.UID)
	}
	if t, ok := utils.ParseTimeBoundary(params.StartTime); ok {
		query.AddCondition("created_at >= ?", t)
	}
	if t, ok := utils.ParseTimeBoundary(params.EndTime); ok {
		query.AddCondition("created_at <= ?", t)
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
		return nil, err
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection), nil
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

	remainingTime := time.Duration(0)
	if loginLog.TokenExpires != nil {
		remainingTime = time.Until(loginLog.TokenExpires.Time)
	}
	if remainingTime <= 0 {
		s.kickSession(loginLog.UID, loginLog.JwtID, SessionControlCodeKicked, revokeReason)
		return nil
	}
	if err := s.writeTokenToBlacklistFn(loginLog.JwtID, remainingTime); err != nil {
		log.Logger.Warn("Redis 黑名单写入失败，保留数据库撤销状态作为兜底",
			zap.Error(err),
			zap.Bool("redis_unavailable", errors.Is(err, errRedisUnavailable)),
			zap.Uint("id", id),
			zap.String("jwt_id", loginLog.JwtID))
		s.kickSession(loginLog.UID, loginLog.JwtID, SessionControlCodeKicked, revokeReason)
		return nil
	}
	s.kickSession(loginLog.UID, loginLog.JwtID, SessionControlCodeKicked, revokeReason)
	return nil
}

func (s *LoginService) loadRevocableSession(id uint) (*model.AdminLoginLogs, error) {
	loginLog := model.NewAdminLoginLogs()
	if s.loginLogDB != nil {
		loginLog.SetDB(s.loginLogDB)
	}
	if err := loginLog.GetById(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.NotFound)
		}
		log.Logger.Error("查询在线会话失败", zap.Error(err), zap.Uint("id", id))
		return nil, err
	}
	if loginLog.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	if loginLog.LoginStatus != model.LoginStatusSuccess {
		return nil, e.NewBusinessError(e.InvalidParameter, "仅成功登录会话允许撤销")
	}
	if loginLog.IsRevoked != model.IsRevokedNo {
		return nil, e.NewBusinessError(e.InvalidParameter, "会话已撤销")
	}
	now := time.Now()
	accessAlive := loginLog.TokenExpires != nil && loginLog.TokenExpires.Time.After(now)
	refreshAlive := loginLog.RefreshExpires != nil && loginLog.RefreshExpires.Time.After(now)
	if !accessAlive && !refreshAlive {
		return nil, e.NewBusinessError(e.InvalidParameter, "会话已过期")
	}
	if loginLog.JwtID == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "会话缺少 jwt_id")
	}
	return loginLog, nil
}
