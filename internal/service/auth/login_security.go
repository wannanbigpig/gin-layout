package auth

import (
	stderrors "errors"
	"math"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/service/sys_config"
	"go.uber.org/zap"
)

const (
	defaultLoginMaxFailures = 5
	defaultLoginLockMinutes = 15
)

type loginLockPolicy struct {
	Enabled      bool
	MaxFailures  int
	LockDuration time.Duration
}

// CheckLoginAllowed 校验账号当前是否允许继续登录。
func (s *LoginService) CheckLoginAllowed(username string) error {
	return s.ensureLoginAllowed(username)
}

// HandleLoginFailure 统一处理登录失败：记录失败日志，并按策略累加失败计数。
// 当 countTowardLock 为 true 且失败计数更新遇到真实数据库错误时，返回 error 以触发 fail-closed。
// 表不存在或依赖未就绪等部署阶段问题不会阻断登录流程。
func (s *LoginService) HandleLoginFailure(username, failReason string, logInfo LoginLogInfo, countTowardLock bool) error {
	s.RecordLoginFailLog(username, failReason, logInfo)
	if !countTowardLock {
		return nil
	}
	if err := s.incrementLoginFailState(username); err != nil {
		if isTableNotFoundErr(err) || e.IsDependencyNotReady(err) {
			log.Logger.Warn("更新登录失败计数失败（表或依赖未就绪）", zap.String("username", username), zap.Error(err))
			return nil
		}
		log.Logger.Warn("更新登录失败计数失败", zap.String("username", username), zap.Error(err))
		return err
	}
	return nil
}

func (s *LoginService) ensureLoginAllowed(username string) error {
	policy := s.loginLockPolicy()
	if !policy.Enabled {
		return nil
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}

	state := model.NewLoginSecurityState()
	if s.loginLogDB != nil {
		state.SetDB(s.loginLogDB)
	}
	err := state.FindByUsername(username)
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) || isTableNotFoundErr(err) || e.IsDependencyNotReady(err) {
			return nil
		}
		return err
	}

	now := time.Now()
	if state.LockUntil == nil {
		return nil
	}
	if !state.LockUntil.Time.After(now) {
		// 锁已过期后清空状态，避免残留失败计数影响后续判断。
		if state.FailCount > 0 || state.LockUntil != nil {
			if clearErr := s.clearLoginFailState(username); clearErr != nil {
				log.Warn("清除登录锁定状态失败", zap.String("username", username), zap.Error(clearErr))
			}
		}
		return nil
	}

	remainingMinutes := int(math.Ceil(state.LockUntil.Time.Sub(now).Minutes()))
	if remainingMinutes < 1 {
		remainingMinutes = 1
	}
	return e.NewBusinessErrorWithKey(e.LoginAccountLocked, e.MsgKeyAuthAccountLocked, remainingMinutes)
}

func (s *LoginService) incrementLoginFailState(username string) error {
	policy := s.loginLockPolicy()
	if !policy.Enabled {
		return nil
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}

	state := model.NewLoginSecurityState()
	if s.loginLogDB != nil {
		state.SetDB(s.loginLogDB)
	}
	lockMinutes := int(policy.LockDuration.Minutes())
	if lockMinutes < 1 {
		lockMinutes = defaultLoginLockMinutes
	}
	err := state.IncrementFailCount(username, policy.MaxFailures, lockMinutes)
	if err != nil {
		if isTableNotFoundErr(err) || e.IsDependencyNotReady(err) {
			return nil
		}
		return err
	}
	return nil
}

func (s *LoginService) clearLoginFailState(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}
	state := model.NewLoginSecurityState()
	if s.loginLogDB != nil {
		state.SetDB(s.loginLogDB)
	}
	db, err := state.GetDB()
	if err != nil {
		if e.IsDependencyNotReady(err) {
			return nil
		}
		return err
	}
	return db.Model(state).Where("username = ?", username).Updates(map[string]any{
		"fail_count":     0,
		"lock_until":     nil,
		"last_failed_at": nil,
	}).Error
}

func (s *LoginService) loginLockPolicy() loginLockPolicy {
	policy := loginLockPolicy{
		Enabled:      sys_config.BoolValue(sys_config.AuthLoginLockEnabledConfigKey, true),
		MaxFailures:  sys_config.IntValue(sys_config.AuthLoginMaxFailuresConfigKey, defaultLoginMaxFailures),
		LockDuration: time.Duration(sys_config.IntValue(sys_config.AuthLoginLockMinutesConfigKey, defaultLoginLockMinutes)) * time.Minute,
	}
	if policy.MaxFailures < 1 {
		policy.MaxFailures = defaultLoginMaxFailures
	}
	if policy.LockDuration < time.Minute {
		policy.LockDuration = defaultLoginLockMinutes * time.Minute
	}
	return policy
}

func (s *LoginService) shouldCountLockFailure(err error) bool {
	var businessErr *e.BusinessError
	if !stderrors.As(err, &businessErr) {
		return false
	}
	switch businessErr.GetCode() {
	case e.UserDoesNotExist, e.UserDisable, e.UserPasswordWrong, e.CaptchaErr:
		return true
	default:
		return false
	}
}

func applyLoginFailState(state *model.LoginSecurityState, now time.Time, policy loginLockPolicy) {
	if state == nil {
		return
	}
	if state.LockUntil != nil && !state.LockUntil.Time.After(now) {
		state.FailCount = 0
		state.LockUntil = nil
	}

	state.FailCount++
	state.LastFailedAt = &utils.FormatDate{Time: now}
	if int(state.FailCount) >= policy.MaxFailures {
		state.LockUntil = &utils.FormatDate{Time: now.Add(policy.LockDuration)}
	}
}

func isTableNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	// MySQL error 1146: table doesn't exist
	var mysqlErr *mysql.MySQLError
	if stderrors.As(err, &mysqlErr) && mysqlErr.Number == 1146 {
		return true
	}
	// SQLite: "no such table"
	// PostgreSQL: "relation does not exist"
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") || strings.Contains(msg, "relation does not exist")
}
