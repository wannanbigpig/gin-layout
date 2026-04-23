package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

const migrationsPathEnvKey = "GO_LAYOUT_MIGRATIONS_PATH"

// ResetService 保留历史入口，对外统一暴露系统维护能力。
// 当前不持有状态，仅为兼容旧调用保留。
type ResetService struct {
	// configProvider 提供运行时配置读取入口。
	configProvider func() *config.Conf
}

// NewResetService 创建兼容旧调用的系统维护服务。
func NewResetService() *ResetService {
	return NewResetServiceWithDeps(ResetServiceDeps{})
}

// ResetServiceDeps 描述 ResetService 可注入依赖。
type ResetServiceDeps struct {
	// ConfigProvider 自定义配置读取函数。
	ConfigProvider func() *config.Conf
}

// NewResetServiceWithDeps 创建带依赖注入的系统维护服务实例。
func NewResetServiceWithDeps(deps ResetServiceDeps) *ResetService {
	s := &ResetService{
		configProvider: deps.ConfigProvider,
	}
	s.ensureRuntimeDeps()
	return s
}

func (s *ResetService) ensureRuntimeDeps() {
	if s.configProvider == nil {
		s.configProvider = config.GetConfig
	}
}

func (s *ResetService) currentConfig() *config.Conf {
	s.ensureRuntimeDeps()
	return config.GetConfigFrom(s.configProvider)
}

// ResetSystemData 兼容旧入口，实际执行日常清理任务。
func (s *ResetService) ResetSystemData() error {
	return s.cleanupExpiredSystemData()
}

// ReinitializeSystemData 兼容旧入口，实际执行系统重建任务。
func (s *ResetService) ReinitializeSystemData() error {
	return s.reinitializeSystemData()
}

// ResetSystemData 清理过期日志与已撤销 token 记录。
func ResetSystemData() error {
	return NewResetService().ResetSystemData()
}

// ReinitializeSystemData 重新初始化系统数据。
func ReinitializeSystemData() error {
	return NewResetService().ReinitializeSystemData()
}

func (s *ResetService) cleanupExpiredSystemData() error {
	db := data.MysqlDB()
	if db == nil {
		err := model.ErrDBUninitialized
		if initErr := data.MysqlInitError(); initErr != nil {
			err = fmt.Errorf("%w: %v", model.ErrDBUninitialized, initErr)
		}
		log.Logger.Error("数据库连接未初始化", zap.Error(err))
		return err
	}

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	log.Logger.Info("开始执行系统日常清理", zap.String("cutoff_date", thirtyDaysAgo.Format("2006-01-02 15:04:05")))

	var deletedRequestLogs, deletedLoginLogs, deletedRevokedTokens int64

	requestLogs := model.NewRequestLogs()
	result := db.Model(requestLogs).
		Where("created_at < ?", thirtyDaysAgo).
		Delete(requestLogs)
	if result.Error != nil {
		log.Logger.Error("清理请求日志失败", zap.Error(result.Error))
	} else {
		deletedRequestLogs = result.RowsAffected
		log.Logger.Info("清理请求日志完成", zap.Int64("deleted_count", deletedRequestLogs))
	}

	loginLogs := model.NewAdminLoginLogs()
	result = db.Model(loginLogs).
		Where("created_at < ?", thirtyDaysAgo).
		Delete(loginLogs)
	if result.Error != nil {
		log.Logger.Error("清理登录日志失败", zap.Error(result.Error))
	} else {
		deletedLoginLogs = result.RowsAffected
		log.Logger.Info("清理登录日志完成", zap.Int64("deleted_count", deletedLoginLogs))
	}

	result = db.Model(loginLogs).
		Where("is_revoked = 1 AND revoked_at < ?", thirtyDaysAgo).
		Delete(loginLogs)
	if result.Error != nil {
		log.Logger.Error("清理已撤销Token失败", zap.Error(result.Error))
	} else {
		deletedRevokedTokens = result.RowsAffected
		log.Logger.Info("清理已撤销Token完成", zap.Int64("deleted_count", deletedRevokedTokens))
	}

	log.Logger.Info("系统日常清理完成",
		zap.Int64("deleted_request_logs", deletedRequestLogs),
		zap.Int64("deleted_login_logs", deletedLoginLogs),
		zap.Int64("deleted_revoked_tokens", deletedRevokedTokens),
	)
	return nil
}

func (s *ResetService) reinitializeSystemData() error {
	log.Logger.Info("开始重新初始化系统数据")

	if err := s.rollbackMigrations(); err != nil {
		log.Logger.Error("回滚迁移失败", zap.Error(err))
		return fmt.Errorf("回滚迁移失败: %w", err)
	}
	log.Logger.Info("回滚迁移完成")

	if err := s.runMigrations(); err != nil {
		log.Logger.Error("执行迁移失败", zap.Error(err))
		return fmt.Errorf("执行迁移失败: %w", err)
	}
	log.Logger.Info("执行迁移完成")

	if err := initAPIRoutes(); err != nil {
		log.Logger.Error("初始化API路由失败", zap.Error(err))
		return fmt.Errorf("初始化API路由失败: %w", err)
	}
	log.Logger.Info("初始化API路由完成")

	if err := rebuildUserPermissions(); err != nil {
		log.Logger.Error("重建用户最终 API 权限失败", zap.Error(err))
		return fmt.Errorf("重建用户最终 API 权限失败: %w", err)
	}
	log.Logger.Info("重建用户最终 API 权限完成")

	log.Logger.Info("系统数据重新初始化完成")
	return nil
}

func (s *ResetService) rollbackMigrations() error {
	m, err := s.createMigrateInstance()
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		var dirtyErr migrate.ErrDirty
		if errors.As(err, &dirtyErr) {
			log.Logger.Warn("检测到 dirty 迁移状态，尝试自动修复并重试回滚", zap.Uint("version", uint(dirtyErr.Version)))
			if forceErr := m.Force(int(dirtyErr.Version)); forceErr != nil {
				return fmt.Errorf("自动修复 dirty 状态失败: %w", forceErr)
			}
			if retryErr := m.Down(); retryErr != nil && retryErr != migrate.ErrNoChange {
				return retryErr
			}
			return nil
		}
		return err
	}
	return nil
}

func (s *ResetService) runMigrations() error {
	m, err := s.createMigrateInstance()
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func (s *ResetService) createMigrateInstance() (*migrate.Migrate, error) {
	migrationsPath, err := getMigrationsPath()
	if err != nil {
		return nil, fmt.Errorf("获取迁移文件路径失败: %w", err)
	}

	dbURL := s.buildDatabaseURL()
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		dbURL,
	)
	if err != nil {
		return nil, fmt.Errorf("创建迁移实例失败: %w", err)
	}
	return m, nil
}

func getMigrationsPath() (string, error) {
	possiblePaths := []string{}

	if envPath := strings.TrimSpace(os.Getenv(migrationsPathEnvKey)); envPath != "" {
		possiblePaths = append(possiblePaths, strings.TrimPrefix(envPath, "file://"))
	}

	if config.V != nil {
		configPath := strings.TrimSpace(config.V.ConfigFileUsed())
		if configPath != "" {
			possiblePaths = append(possiblePaths, filepath.Join(filepath.Dir(configPath), "data", "migrations"))
		}
	}
	if executablePath, err := os.Executable(); err == nil {
		possiblePaths = append(possiblePaths, filepath.Join(filepath.Dir(executablePath), "data", "migrations"))
	}
	if _, currentFile, _, ok := runtime.Caller(0); ok {
		possiblePaths = append(possiblePaths, filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "data", "migrations"))
	}
	possiblePaths = append(possiblePaths,
		"data/migrations",
		"./data/migrations",
		"../data/migrations",
		"../../data/migrations",
	)

	seen := make(map[string]struct{}, len(possiblePaths))
	for _, path := range possiblePaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, ok := seen[absPath]; ok {
			continue
		}
		seen[absPath] = struct{}{}
		matches, err := filepath.Glob(filepath.Join(absPath, "*.up.sql"))
		if err == nil && len(matches) > 0 {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("未找到迁移文件目录，请确保 data/migrations 目录存在，或通过环境变量 %s 指定路径", migrationsPathEnvKey)
}

func (s *ResetService) buildDatabaseURL() string {
	cfg := s.currentConfig().Mysql
	return fmt.Sprintf("mysql://%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)
}

func initAPIRoutes() error {
	return InitApiRoutes()
}

func rebuildUserPermissions() error {
	return RebuildUserPermissions()
}
