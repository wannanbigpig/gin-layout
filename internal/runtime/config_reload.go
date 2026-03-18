package runtime

import (
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

var registerOnce sync.Once

// RegisterConfigReloadHandlers 注册配置热更新处理器。
func RegisterConfigReloadHandlers() {
	registerOnce.Do(func() {
		config.RegisterConfigReloadHandler(config.ConfigReloadHandler{
			Name:     "logger",
			Priority: 10,
			Handle:   reloadLogger,
		})
		config.RegisterConfigReloadHandler(config.ConfigReloadHandler{
			Name:     "data",
			Priority: 20,
			Handle:   reloadData,
		})
		config.RegisterConfigReloadHandler(config.ConfigReloadHandler{
			Name:     "casbin",
			Priority: 30,
			Handle:   reloadCasbin,
		})
		config.RegisterConfigReloadHandler(config.ConfigReloadHandler{
			Name:     "warnings",
			Priority: 100,
			Handle:   logWarnings,
		})
	})
}

func reloadLogger(oldConfig, newConfig *config.Conf, diff config.ConfigDiff) error {
	if !diff.LoggerChanged {
		return nil
	}
	return log.ReloadLogger(newConfig)
}

func reloadData(oldConfig, newConfig *config.Conf, diff config.ConfigDiff) error {
	if diff.MysqlChanged {
		if err := data.ReloadMysql(newConfig); err != nil {
			return fmt.Errorf("mysql reload failed: %w", err)
		}
		log.Logger.Info("MySQL runtime reloaded")
	}
	if diff.RedisChanged {
		if err := data.ReloadRedis(newConfig); err != nil {
			return fmt.Errorf("redis reload failed: %w", err)
		}
		log.Logger.Info("Redis runtime reloaded")
	}
	return nil
}

func reloadCasbin(oldConfig, newConfig *config.Conf, diff config.ConfigDiff) error {
	if !diff.MysqlChanged {
		return nil
	}
	if !newConfig.Mysql.Enable {
		return nil
	}
	if err := casbinx.ReloadEnforcer(); err != nil {
		return fmt.Errorf("casbin reload failed: %w", err)
	}
	log.Logger.Info("Casbin runtime reloaded")
	return nil
}

func logWarnings(oldConfig, newConfig *config.Conf, diff config.ConfigDiff) error {
	logConfigDiff(diff)
	if len(diff.RestartRequiredFields) > 0 {
		log.Logger.Warn("Detected config changes that require process restart",
			zap.Strings("fields", diff.RestartRequiredFields),
		)
	}
	return nil
}

func logConfigDiff(diff config.ConfigDiff) {
	if len(diff.ChangedFields) == 0 {
		return
	}
	log.Logger.Info("Detected config changes",
		zap.Strings("fields", diff.ChangedFields),
	)
}
