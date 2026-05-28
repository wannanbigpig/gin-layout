package auth

import (
	"context"
	stderrors "errors"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

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
	loginLogQueueOnce sync.Once
	loginLogQueue     chan *model.AdminLoginLogs
)

// initLoginLogWorker 懒启动单 worker，从队列拉取并落库登录日志。
// 单 worker 已足够：登录链路 QPS 远低于落库吞吐，且单写入串行化便于失败时降级。
func initLoginLogWorker() {
	loginLogQueueOnce.Do(func() {
		loginLogQueue = make(chan *model.AdminLoginLogs, loginLogAsyncBufferSize)
		go runLoginLogWorker(loginLogQueue)
	})
}

func runLoginLogWorker(queue <-chan *model.AdminLoginLogs) {
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
	select {
	case loginLogQueue <- entry:
		return true
	default:
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
	tokenRevokedLookupFn func(jwtID string) bool
	// mysqlReadyFn 判断 MySQL 依赖是否可用。
	mysqlReadyFn func() bool
	// refreshLockStore 在 Redis 不可用时提供进程内刷新锁。
	refreshLockStore *refreshTokenLock
	// tryRefreshPrincipalFn 执行 principal 自动刷新逻辑。
	tryRefreshPrincipalFn func(principal *AuthPrincipal)
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
	TokenRevokedLookup func(jwtID string) bool
	// MySQLReady 自定义 MySQL 可用性判断逻辑。
	MySQLReady func() bool
	// RefreshLockStore 自定义内存刷新锁存储实现。
	RefreshLockStore *refreshTokenLock
	// TryRefreshPrincipal 自定义自动刷新 principal 的执行入口。
	TryRefreshPrincipal func(principal *AuthPrincipal)
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
		refreshLockStore:        deps.RefreshLockStore,
		tryRefreshPrincipalFn:   deps.TryRefreshPrincipal,
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
	if s.refreshLockStore == nil {
		s.refreshLockStore = defaultRefreshLockStore()
	}
	if s.tryRefreshPrincipalFn == nil {
		s.tryRefreshPrincipalFn = s.tryRefreshToken
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
		s.HandleLoginFailure(username, s.extractErrorMessage(err), logInfo, false)
		return nil, err
	}

	adminUser, err := s.validateUser(username, password)
	if err != nil {
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		s.HandleLoginFailure(username, s.extractErrorMessage(err), logInfo, s.shouldCountLockFailure(err))
		return nil, err
	}

	claims := s.newAdminCustomClaims(adminUser)
	accessToken, err := token.Generate(claims)
	if err != nil {
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		s.HandleLoginFailure(username, "生成Token失败", logInfo, false)
		return nil, e.NewBusinessError(e.TokenGenerateFailed)
	}

	logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
	if err := s.recordLoginLog(adminUser, claims, accessToken, "", logInfo, model.LoginTypeLogin); err != nil {
		return nil, e.NewBusinessError(e.LoginFailed)
	}
	if err := s.clearLoginFailState(username); err != nil && !isTableNotFoundErr(err) {
		log.Logger.Warn("清理登录失败计数失败", zap.String("username", username), zap.Error(err))
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "",
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

// recordLoginLog 记录登录日志并更新用户信息。
//
// 关键路径变更：
//   - 同步阶段仅保留 admin_users 的 last_ip / last_login 字段更新（使用 Updates(map) 显式指定列，
//     避免 Save 全字段回写带来的写放大与并发覆盖风险）；
//   - admin_login_logs 的 INSERT 改为投递异步队列，登录响应不再被这条 I/O 拖慢；
//   - 异步队列已满或尚未初始化时回退同步写入，保证不丢日志；任意路径失败均必须打 Error 日志。
func (s *LoginService) recordLoginLog(adminUser *model.AdminUser, claims token.AdminCustomClaims, accessToken, refreshToken string, logInfo LoginLogInfo, logType uint8) error {
	loginLog := s.buildLoginLog(adminUser.ID, adminUser.Username, claims.ID, accessToken, refreshToken, claims.ExpiresAt.Time, logInfo, model.LoginStatusSuccess, "", logType)

	if logType == model.LoginTypeLogin {
		updates := map[string]any{
			"last_ip":    logInfo.IP,
			"last_login": utils.FormatDate{Time: time.Now()},
		}
		if err := model.NewAdminUsers().UpdateById(adminUser.ID, updates); err != nil {
			log.Logger.Error("更新用户最后登录信息失败", zap.Error(err), zap.Uint("user_id", adminUser.ID))
			return err
		}
	}

	if enqueueLoginLogAsync(loginLog) {
		return nil
	}

	// 队列不可用时同步落库兜底，避免登录日志丢失。
	if err := loginLog.Create(); err != nil {
		log.Logger.Error("记录登录日志失败", zap.Error(err), zap.Uint("user_id", adminUser.ID), zap.String("username", adminUser.Username))
		return err
	}
	return nil
}

// Refresh 刷新 Token。
func (s *LoginService) Refresh(id uint, logInfo LoginLogInfo) (*TokenResponse, error) {
	startTime := time.Now()

	adminUserModel := model.NewAdminUsers()
	if err := adminUserModel.GetById(id); err != nil {
		return nil, e.NewBusinessError(e.UpdateUserFailed)
	}
	if adminUserModel.Status != model.AdminUserStatusEnabled {
		return nil, e.NewBusinessError(e.UserDisable)
	}

	claims := s.newAdminCustomClaims(adminUserModel)
	accessToken, err := token.Refresh(claims)
	if err != nil {
		return nil, e.NewBusinessError(e.TokenGenerateFailed)
	}

	logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
	if err := s.recordLoginLog(adminUserModel, claims, accessToken, "", logInfo, model.LoginTypeRefresh); err != nil {
		log.Logger.Error("记录刷新token日志失败", zap.Error(err), zap.Uint("user_id", id))
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "",
		TokenType:    tokenTypeBearer,
		ExpiresAt:    claims.ExpiresAt.Unix(),
	}, nil
}

// newAdminCustomClaims 创建管理员自定义 Claims。
func (s *LoginService) newAdminCustomClaims(user *model.AdminUser) token.AdminCustomClaims {
	return token.NewAdminCustomClaims(user)
}
