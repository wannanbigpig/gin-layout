package bootstrapx

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"go.uber.org/zap"
)

const errorLoadingLocation = "Error loading location: %v"

// Requirements 描述命令运行前需要初始化的依赖。
type Requirements struct {
	Data      bool
	Validator bool
}

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
func InitializeLogger() {
	log.InitLogger()
}

// InitializeData 初始化数据源依赖。
func InitializeData() error {
	data.InitData()
	return nil
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
			if err := InitializeData(); err != nil {
				return err
			}
		}
		if req.Validator {
			if err := InitializeValidator(); err != nil {
				return err
			}
		}
		if originalPreRunE != nil {
			return originalPreRunE(c, args)
		}
		return nil
	}

	return cmd
}
