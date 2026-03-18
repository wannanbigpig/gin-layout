package system

import (
	"fmt"
	"path/filepath"
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

// ResetService 系统重置服务
type ResetService struct{}

// NewResetService 创建系统重置服务实例
func NewResetService() *ResetService {
	return &ResetService{}
}

// ResetSystemData 重置系统数据
// 每天凌晨执行，清理旧的日志数据，保留最近30天的数据
func (s *ResetService) ResetSystemData() error {
	db := data.MysqlDB()
	if db == nil {
		log.Logger.Error("数据库连接未初始化")
		return nil
	}

	// 计算30天前的时间
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	log.Logger.Info("开始重置系统数据", zap.String("cutoff_date", thirtyDaysAgo.Format("2006-01-02 15:04:05")))

	var deletedRequestLogs, deletedLoginLogs, deletedRevokedTokens int64

	// 1. 清理30天前的请求日志
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

	// 2. 清理30天前的登录日志（软删除的数据也会被清理）
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

	// 3. 清理30天前已撤销的Token记录（可选，根据业务需求）
	// 注意：这里只清理已撤销且过期的Token，保留未撤销的Token用于审计
	result = db.Model(loginLogs).
		Where("is_revoked = 1 AND revoked_at < ?", thirtyDaysAgo).
		Delete(loginLogs)
	if result.Error != nil {
		log.Logger.Error("清理已撤销Token失败", zap.Error(result.Error))
	} else {
		deletedRevokedTokens = result.RowsAffected
		log.Logger.Info("清理已撤销Token完成", zap.Int64("deleted_count", deletedRevokedTokens))
	}

	// 4. 清理过期的验证码数据（如果有存储的话）
	// 验证码通常在Redis中，这里可以清理Redis中的过期验证码
	// 由于验证码有自动过期机制，这里主要是确保清理

	log.Logger.Info("系统数据重置完成",
		zap.Int64("deleted_request_logs", deletedRequestLogs),
		zap.Int64("deleted_login_logs", deletedLoginLogs),
		zap.Int64("deleted_revoked_tokens", deletedRevokedTokens),
	)

	return nil
}

// ReinitializeSystemData 重新初始化系统数据
// 1. 回滚所有迁移
// 2. 重新执行迁移
// 3. 重新初始化API路由
// 4. 全量重建用户最终 API 权限
func (s *ResetService) ReinitializeSystemData() error {
	log.Logger.Info("开始重新初始化系统数据")

	// 步骤1: 回滚所有迁移
	if err := s.rollbackMigrations(); err != nil {
		log.Logger.Error("回滚迁移失败", zap.Error(err))
		return fmt.Errorf("回滚迁移失败: %w", err)
	}
	log.Logger.Info("回滚迁移完成")

	// 步骤2: 重新执行迁移
	if err := s.runMigrations(); err != nil {
		log.Logger.Error("执行迁移失败", zap.Error(err))
		return fmt.Errorf("执行迁移失败: %w", err)
	}
	log.Logger.Info("执行迁移完成")

	// 步骤3: 重新初始化API路由
	if err := s.initApiRoutes(); err != nil {
		log.Logger.Error("初始化API路由失败", zap.Error(err))
		return fmt.Errorf("初始化API路由失败: %w", err)
	}
	log.Logger.Info("初始化API路由完成")

	// 步骤4: 全量重建用户最终 API 权限
	if err := s.rebuildUserPermissions(); err != nil {
		log.Logger.Error("重建用户最终 API 权限失败", zap.Error(err))
		return fmt.Errorf("重建用户最终 API 权限失败: %w", err)
	}
	log.Logger.Info("重建用户最终 API 权限完成")

	log.Logger.Info("系统数据重新初始化完成")
	return nil
}

// rollbackMigrations 回滚所有迁移
func (s *ResetService) rollbackMigrations() error {
	m, err := s.createMigrateInstance()
	if err != nil {
		return err
	}
	defer m.Close()

	// 回滚到版本0（完全回滚）
	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

// runMigrations 执行所有迁移
func (s *ResetService) runMigrations() error {
	m, err := s.createMigrateInstance()
	if err != nil {
		return err
	}
	defer m.Close()

	// 执行所有迁移
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

// createMigrateInstance 创建迁移实例
func (s *ResetService) createMigrateInstance() (*migrate.Migrate, error) {
	// 获取迁移文件路径
	migrationsPath, err := s.getMigrationsPath()
	if err != nil {
		return nil, fmt.Errorf("获取迁移文件路径失败: %w", err)
	}

	// 构建数据库连接URL
	dbURL := s.buildDatabaseURL()

	// 创建迁移实例
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		dbURL,
	)
	if err != nil {
		return nil, fmt.Errorf("创建迁移实例失败: %w", err)
	}

	return m, nil
}

// getMigrationsPath 获取迁移文件路径
func (s *ResetService) getMigrationsPath() (string, error) {
	// 尝试多个可能的路径
	possiblePaths := []string{
		"data/migrations",
		"./data/migrations",
		"../data/migrations",
		"../../data/migrations",
	}

	for _, path := range possiblePaths {
		if absPath, err := filepath.Abs(path); err == nil {
			// 检查目录是否存在且包含迁移文件
			matches, err := filepath.Glob(filepath.Join(absPath, "*.up.sql"))
			if err == nil && len(matches) > 0 {
				return absPath, nil
			}
		}
	}

	return "", fmt.Errorf("未找到迁移文件目录，请确保 data/migrations 目录存在")
}

// buildDatabaseURL 构建数据库连接URL
func (s *ResetService) buildDatabaseURL() string {
	cfg := config.Config.Mysql
	// 构建 MySQL 连接URL，格式: mysql://user:password@tcp(host:port)/database?params
	return fmt.Sprintf("mysql://%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)
}

// initApiRoutes 初始化API路由（跳过用户确认）
func (s *ResetService) initApiRoutes() error {
	initService := NewInitService()
	return initService.InitApiRoutes()
}

// rebuildUserPermissions 全量重建用户最终 API 权限（跳过用户确认）。
func (s *ResetService) rebuildUserPermissions() error {
	initService := NewInitService()
	return initService.RebuildUserPermissions()
}
