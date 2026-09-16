package runtime

import (
	"fmt"
	"sync"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
)

var registerOnce sync.Once

// RegisterConfigReloadHandlers 注册一次可预备、整体提交的运行时更新。
func RegisterConfigReloadHandlers() {
	registerOnce.Do(func() {
		config.RegisterConfigReloadHandler(config.ConfigReloadHandler{
			Name: "runtime", Priority: 10, Prepare: prepareRuntimeReload,
		})
	})
}

func prepareRuntimeReload(oldConfig, newConfig *config.Conf, diff config.ConfigDiff) (config.ConfigReloadChange, error) {
	var loggerChange config.ConfigReloadChange
	var err error
	if diff.LoggerChanged {
		loggerChange, err = log.PrepareLoggerReload(newConfig)
		if err != nil {
			return config.ConfigReloadChange{}, fmt.Errorf("logger prepare failed: %w", err)
		}
	}
	preparedData, err := data.PrepareReload(newConfig, diff)
	if err != nil {
		if loggerChange.Abort != nil {
			loggerChange.Abort()
		}
		return config.ConfigReloadChange{}, err
	}
	abort := func() {
		preparedData.Abort()
		if loggerChange.Abort != nil {
			loggerChange.Abort()
		}
	}
	var commitEnforcer func()
	if diff.MysqlChanged {
		commitEnforcer, err = casbinx.PrepareEnforcerReload(preparedData.MysqlDB())
		if err != nil {
			abort()
			return config.ConfigReloadChange{}, fmt.Errorf("casbin prepare failed: %w", err)
		}
	}
	return config.ConfigReloadChange{
		Commit: func() {
			preparedData.Commit()
			if commitEnforcer != nil {
				commitEnforcer()
			}
			if loggerChange.Commit != nil {
				loggerChange.Commit()
			}
		},
		Abort: abort,
		Finalize: func() {
			preparedData.Finalize()
			if loggerChange.Finalize != nil {
				loggerChange.Finalize()
			}
			logWarnings(diff)
		},
	}, nil
}

func logWarnings(diff config.ConfigDiff) {
	if len(diff.ChangedFields) > 0 {
		log.Logger.Info("Applied config changes", zap.Strings("fields", diff.ChangedFields))
	}
	if len(diff.RestartRequiredFields) > 0 {
		log.Logger.Warn("Detected config changes that require process restart", zap.Strings("fields", diff.RestartRequiredFields))
	}
}
