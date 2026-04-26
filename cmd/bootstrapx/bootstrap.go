package bootstrapx

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"github.com/wannanbigpig/gin-layout/internal/service/sys_config"
	"github.com/wannanbigpig/gin-layout/internal/service/system"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"go.uber.org/zap"
)

const errorLoadingLocation = "Error loading location: %v"

// Requirements 描述命令运行前需要初始化的依赖。
type Requirements struct {
	Data                 bool
	Validator            bool
	Queue                bool
	AllowDegradedStartup bool
}

var (
	initializeDataFunc      = InitializeData
	initializeValidatorFunc = InitializeValidator
	initializeQueueFunc     = InitializeQueue
)

// InitializeConfig 初始化配置。
func InitializeConfig(configPath string) error {
	return config.InitConfig(configPath)
}

// InitializeTimezone 根据配置设置进程时区。
func InitializeTimezone() {
	cfg := config.GetConfig()
	if cfg.Timezone == nil {
		return
	}

	location, err := time.LoadLocation(*cfg.Timezone)
	if err != nil {
		if log.Logger != nil {
			log.Logger.Error(fmt.Sprintf(errorLoadingLocation, err), zap.Error(err))
		}
		fmt.Printf(errorLoadingLocation+"\n", err)
		return
	}
	time.Local = location
}

// InitializeLogger 初始化全局日志组件。
func InitializeLogger() error {
	return log.InitLogger()
}

// InitializeData 初始化数据源依赖。
func InitializeData() error {
	if err := data.InitData(); err != nil {
		return err
	}
	taskcron.RegisterHandler(taskcron.HandlerCronResetSystemData, func(ctx context.Context, payload map[string]any) error {
		_ = ctx
		_ = payload
		return system.ReinitializeSystemData()
	})
	if err := sys_config.NewSysConfigService().WarmupRuntimeConfigIfAvailable(); err != nil {
		return err
	}
	return taskcron.SyncBuiltinDefinitionsIfAvailable(config.GetConfig())
}

// InitializeValidator 初始化参数校验器。
func InitializeValidator() error {
	return validator.InitValidatorTrans("zh")
}

// WrapCommand 为命令注入统一的初始化逻辑，并保留原有 PreRunE/RunE。
func WrapCommand(cmd *cobra.Command, req Requirements) *cobra.Command {
	if cmd == nil {
		return cmd
	}

	originalPreRunE := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		if req.Data {
			if err := initializeDataFunc(); err != nil {
				if shouldAllowDegradedStartup(req) {
					logDependencyInitWarning(c, "data", err)
				} else {
					return err
				}
			}
		}
		if req.Validator {
			if err := initializeValidatorFunc(); err != nil {
				return err
			}
		}
		if req.Queue {
			if err := initializeQueueFunc(); err != nil {
				if shouldAllowDegradedStartup(req) {
					logDependencyInitWarning(c, "queue", err)
				} else {
					return err
				}
			}
		}
		if originalPreRunE != nil {
			return originalPreRunE(c, args)
		}
		return nil
	}

	return cmd
}

// InitializeQueue 初始化队列发布者。
func InitializeQueue() error {
	cfg := config.GetConfig()
	if !cfg.Queue.Enable {
		return nil
	}
	if err := queue.InitPublisher(cfg); err != nil {
		return err
	}
	return queue.InitInspector(cfg)
}

func shouldAllowDegradedStartup(req Requirements) bool {
	if !req.AllowDegradedStartup {
		return false
	}
	cfg := config.GetConfig()
	return cfg != nil && cfg.AllowDegradedStartup
}

func logDependencyInitWarning(cmd *cobra.Command, dependency string, err error) {
	if err == nil {
		return
	}
	commandPath := ""
	if cmd != nil {
		commandPath = cmd.CommandPath()
	}
	if log.Logger != nil {
		log.Logger.Warn("Dependency initialization failed, continue with degraded startup",
			zap.String("command", commandPath),
			zap.String("dependency", dependency),
			zap.Error(err))
		return
	}
	fmt.Printf("warning: dependency initialization failed, continue with degraded startup; command=%s dependency=%s err=%v\n", commandPath, dependency, err)
}
