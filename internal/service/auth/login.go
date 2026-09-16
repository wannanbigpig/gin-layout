package auth

import (
	"context"
	stderrors "errors"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// loginLogAsyncBufferSize 控制登录日志异步写入队列容量；队列打满时调用方会被强制走同步落库，
// 避免登录链路被异步队列阻塞。
const loginLogAsyncBufferSize = 256

var (
	loginLogQueueMu     sync.RWMutex
	loginLogQueueOnce   sync.Once
	loginLogQueue       chan *model.AdminLoginLogs
	loginLogWorkerWg    sync.WaitGroup
	loginLogWorkerState struct {
		stopped bool
	}
)

// initLoginLogWorker 懒启动单 worker，从队列拉取并落库登录日志。
// 单 worker 已足够：登录链路 QPS 远低于落库吞吐，且单写入串行化便于失败时降级。
func initLoginLogWorker() {
	loginLogQueueOnce.Do(func() {
		loginLogQueueMu.Lock()
		defer loginLogQueueMu.Unlock()
		if loginLogWorkerState.stopped {
			return
		}
		loginLogQueue = make(chan *model.AdminLoginLogs, loginLogAsyncBufferSize)
		loginLogWorkerWg.Add(1)
		go runLoginLogWorker(loginLogQueue)
	})
}

// StopLoginLogWorker 关闭登录日志异步队列并等待 worker 退出。
// 如果 worker 从未启动（懒初始化未触发），则为 no-op。
// timeout 控制等待 worker 处理完剩余条目的最大时长。
func StopLoginLogWorker(timeout time.Duration) {
	loginLogQueueMu.Lock()
	if loginLogWorkerState.stopped {
		loginLogQueueMu.Unlock()
		return
	}
	loginLogWorkerState.stopped = true
	queue := loginLogQueue
	if queue != nil {
		close(queue)
	}
	loginLogQueueMu.Unlock()

	if queue == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		loginLogWorkerWg.Wait()
		close(done)
	}()
	if timeout <= 0 {
		<-done
		return
	}
	select {
	case <-done:
	case <-time.After(timeout):
		if log.Logger != nil {
			log.Logger.Warn("login log async worker drain timed out", zap.Duration("timeout", timeout))
		}
	}
}

func runLoginLogWorker(queue <-chan *model.AdminLoginLogs) {
	defer loginLogWorkerWg.Done()
	defer func() {
		if r := recover(); r != nil && log.Logger != nil {
			log.Logger.Error("login log async worker panic",
				zap.Any("recover", r))
		}
	}()
	for entry := range queue {
		writeLoginLogAsync(entry)
	}
}

func writeLoginLogAsync(entry *model.AdminLoginLogs) {
	defer func() {
		if r := recover(); r != nil && log.Logger != nil {
			log.Logger.Error("login log async write panic",
				zap.Any("recover", r),
				zap.Uint("user_id", entry.UID),
				zap.String("username", entry.Username))
		}
	}()
	if err := entry.Create(); err != nil {
		if log.Logger != nil {
			log.Logger.Error("login log async write failed",
				zap.Error(err),
				zap.Uint("user_id", entry.UID),
				zap.String("username", entry.Username))
		}
	}
}

// enqueueLoginLogAsync 尝试将登录日志投递到异步队列。
// 队列未初始化或已满时返回 false，调用方需走同步落库兜底。
func enqueueLoginLogAsync(entry *model.AdminLoginLogs) bool {
	if entry == nil {
		return true
	}
	initLoginLogWorker()
	loginLogQueueMu.RLock()
	if loginLogWorkerState.stopped || loginLogQueue == nil {
		loginLogQueueMu.RUnlock()
		return false
	}
	select {
	case loginLogQueue <- entry:
		loginLogQueueMu.RUnlock()
		return true
	default:
		loginLogQueueMu.RUnlock()
		return false
	}
}

// LoginService 登录授权服务。
type LoginService struct {
	service.Base
	// configProvider 提供运行时配置读取入口。
	configProvider func() *config.Conf
	// blacklistLookupFn 查询 token 是否在 Redis 黑名单中。
	blacklistLookupFn func(jwtID string) (bool, error)
	// tokenRevokedLookupFn 查询 token 是否在登录日志中已标记撤销。
	tokenRevokedLookupFn func(jwtID string) (bool, error)
	// mysqlReadyFn 判断 MySQL 依赖是否可用。
	mysqlReadyFn func() bool
	// markTokensRevokedFn 批量标记 token 为撤销状态。
	markTokensRevokedFn func(ctx context.Context, jwtIDs []string, revokedCode uint8, revokedReason string) error
	// writeTokenToBlacklistFn 将 token 写入 Redis 黑名单。
	writeTokenToBlacklistFn func(jwtID string, remainingTime time.Duration) error
	// loginLogDB 允许测试或事务场景指定登录日志查询连接。
	loginLogDB *gorm.DB
}

// LoginServiceDeps 描述 LoginService 可注入依赖。
type LoginServiceDeps struct {
	// ConfigProvider 自定义配置读取函数。
	ConfigProvider func() *config.Conf
	// BlacklistLookup 自定义 Redis 黑名单查询实现。
	BlacklistLookup func(jwtID string) (bool, error)
	// TokenRevokedLookup 自定义登录日志撤销状态查询实现。
	TokenRevokedLookup func(jwtID string) (bool, error)
	// MySQLReady 自定义 MySQL 可用性判断逻辑。
	MySQLReady func() bool
	// MarkTokensRevoked 自定义 token 撤销标记逻辑。
	MarkTokensRevoked func(ctx context.Context, jwtIDs []string, revokedCode uint8, revokedReason string) error
	// WriteTokenToBlacklist 自定义写 Redis 黑名单逻辑。
	WriteTokenToBlacklist func(jwtID string, remainingTime time.Duration) error
	// LoginLogDB 自定义登录日志查询连接。
	LoginLogDB *gorm.DB
}

// NewLoginService 创建登录服务实例。
func NewLoginService() *LoginService {
	return NewLoginServiceWithDeps(LoginServiceDeps{})
}

// NewLoginServiceWithDeps 创建带依赖注入的登录服务实例。
func NewLoginServiceWithDeps(deps LoginServiceDeps) *LoginService {
	s := &LoginService{
		configProvider:          deps.ConfigProvider,
		blacklistLookupFn:       deps.BlacklistLookup,
		tokenRevokedLookupFn:    deps.TokenRevokedLookup,
		mysqlReadyFn:            deps.MySQLReady,
		markTokensRevokedFn:     deps.MarkTokensRevoked,
		writeTokenToBlacklistFn: deps.WriteTokenToBlacklist,
		loginLogDB:              deps.LoginLogDB,
	}
	s.ensureRuntimeDeps()
	return s
}

func (s *LoginService) ensureRuntimeDeps() {
	if s.configProvider == nil {
		s.configProvider = config.GetConfig
	}
	if s.blacklistLookupFn == nil {
		s.blacklistLookupFn = s.IsInBlacklist
	}
	if s.tokenRevokedLookupFn == nil {
		s.tokenRevokedLookupFn = s.isTokenRevokedInLog
	}
	if s.mysqlReadyFn == nil {
		s.mysqlReadyFn = data.MysqlReady
	}
	if s.markTokensRevokedFn == nil {
		s.markTokensRevokedFn = func(ctx context.Context, jwtIDs []string, revokedCode uint8, revokedReason string) error {
			return s.markTokensRevoked(ctx, jwtIDs, revokedCode, revokedReason)
		}
	}
	if s.writeTokenToBlacklistFn == nil {
		s.writeTokenToBlacklistFn = func(jwtID string, remainingTime time.Duration) error {
			return s.writeTokenToBlacklist(jwtID, remainingTime)
		}
	}
}

func (s *LoginService) currentConfig() *config.Conf {
	s.ensureRuntimeDeps()
	return config.GetConfigFrom(s.configProvider)
}

// Login 用户登录。
func (s *LoginService) Login(username, password string, logInfo LoginLogInfo) (*TokenResponse, error) {
	startTime := time.Now()
	if err := s.ensureLoginAllowed(username); err != nil {
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		if handleErr := s.HandleLoginFailure(username, s.extractErrorMessage(err), logInfo, false); handleErr != nil {
			return nil, handleErr
		}
		return nil, err
	}

	adminUser, err := s.validateUser(username, password)
	if err != nil {
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		if handleErr := s.HandleLoginFailure(username, s.extractErrorMessage(err), logInfo, s.shouldCountLockFailure(err)); handleErr != nil {
			return nil, handleErr
		}
		return nil, err
	}

	claims := s.newAdminCustomClaims(adminUser)
	accessToken, err := s.generateAccessToken(claims)
	if err != nil {
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		if handleErr := s.HandleLoginFailure(username, "生成Token失败", logInfo, false); handleErr != nil {
			return nil, handleErr
		}
		return nil, e.NewBusinessError(e.TokenGenerateFailed)
	}

	logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
	loginLog, refreshToken, err := s.issueRefreshToken(adminUser, claims, accessToken, logInfo, model.LoginTypeLogin)
	if err != nil {
		return nil, e.NewBusinessError(e.TokenGenerateFailed)
	}
	if err := s.persistLoginLog(loginLog, true, adminUser); err != nil {
		return nil, e.NewBusinessError(e.LoginFailed)
	}
	if err := s.clearLoginFailState(username); err != nil && !isTableNotFoundErr(err) {
		log.Logger.Warn("清理登录失败计数失败", zap.String("username", username), zap.Error(err))
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    tokenTypeBearer,
		ExpiresAt:    claims.ExpiresAt.Unix(),
	}, nil
}

// validateUser 验证用户信息。
func (s *LoginService) validateUser(username, password string) (*model.AdminUser, error) {
	adminUser := model.NewAdminUsers()
	if err := adminUser.GetUserInfo(username); err != nil {
		switch {
		case e.IsDependencyNotReady(err):
			return nil, e.NewDependencyNotReadyError()
		case stderrors.Is(err, gorm.ErrRecordNotFound):
			return nil, e.NewBusinessError(e.UserDoesNotExist)
		default:
			return nil, err
		}
	}
	if adminUser.Status != model.AdminUserStatusEnabled {
		return nil, e.NewBusinessError(e.UserDisable)
	}
	if !utils2.ComparePasswords(adminUser.Password, password) {
		return nil, e.NewBusinessError(e.UserPasswordWrong)
	}
	return adminUser, nil
}

// recordLoginLog 同步保存认证会话；成功响应前必须可刷新、可撤销。
func (s *LoginService) recordLoginLog(adminUser *model.AdminUser, claims token.AdminCustomClaims, accessToken, refreshToken string, logInfo LoginLogInfo, logType uint8) error {
	loginLog := s.buildLoginLog(adminUser.ID, adminUser.Username, claims.ID, accessToken, refreshToken, claims.ExpiresAt.Time, logInfo, model.LoginStatusSuccess, "", logType)
	if s.loginLogDB != nil {
		loginLog.SetDB(s.loginLogDB)
	}
	return s.persistLoginLog(loginLog, logType == model.LoginTypeLogin, adminUser)
}

func (s *LoginService) persistLoginLog(loginLog *model.AdminLoginLogs, updateLastLogin bool, expectedUsers ...*model.AdminUser) error {
	if loginLog == nil {
		return nil
	}
	db := s.loginLogDB
	if db == nil {
		var err error
		db, err = loginLog.GetDB()
		if err != nil {
			return err
		}
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if len(expectedUsers) > 0 && expectedUsers[0] != nil {
			if err := validateAuthenticatedUserSnapshot(tx, expectedUsers[0]); err != nil {
				return err
			}
		}
		if updateLastLogin {
			adminUserModel := model.NewAdminUsers()
			adminUserModel.SetDB(tx)
			if err := adminUserModel.UpdateById(loginLog.UID, map[string]any{
				"last_ip":    loginLog.IP,
				"last_login": utils.FormatDate{Time: time.Now()},
			}); err != nil {
				return err
			}
		}
		loginLog.SetDB(tx)
		return loginLog.Create()
	})
	if err != nil && log.Logger != nil {
		log.Logger.Error("保存登录会话失败", zap.Error(err), zap.Uint("user_id", loginLog.UID))
	}
	return err
}

func validateAuthenticatedUserSnapshot(tx *gorm.DB, expected *model.AdminUser) error {
	if tx == nil || expected == nil || expected.ID == 0 {
		return e.NewBusinessError(e.NotLogin)
	}
	var current struct {
		ID           uint
		Username     string
		Password     string
		Status       uint8
		IsSuperAdmin uint8
	}
	if err := tx.Model(&model.AdminUser{}).
		Select("id", "username", "password", "status", "is_super_admin").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at = 0", expected.ID).
		Take(&current).Error; err != nil {
		return err
	}
	if current.Status != model.AdminUserStatusEnabled ||
		current.Username != expected.Username ||
		current.Password != expected.Password ||
		current.IsSuperAdmin != expected.IsSuperAdmin {
		return e.NewBusinessError(e.NotLogin)
	}
	return nil
}

func loadAdminUserForAuthentication(tx *gorm.DB, userID uint) (*model.AdminUser, error) {
	if tx == nil || userID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	user := model.NewAdminUsers()
	if err := tx.Model(user).
		Select("id", "username", "status", "is_super_admin").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at = 0", userID).
		Take(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// newAdminCustomClaims 创建管理员自定义 Claims。
func (s *LoginService) newAdminCustomClaims(user *model.AdminUser) token.AdminCustomClaims {
	return token.NewAdminCustomClaims(user)
}
