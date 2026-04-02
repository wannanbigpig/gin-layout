package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/cmd/bootstrapx"
	"github.com/wannanbigpig/gin-layout/cmd/command"
	"github.com/wannanbigpig/gin-layout/cmd/cron"
	"github.com/wannanbigpig/gin-layout/cmd/service"
	"github.com/wannanbigpig/gin-layout/cmd/version"
	"github.com/wannanbigpig/gin-layout/cmd/worker"
	"github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	_ "github.com/wannanbigpig/gin-layout/internal/queue/asynqx"
	"github.com/wannanbigpig/gin-layout/internal/runtime"
)

const (
	welcomeMessage = "Welcome to go-layout. Use -h to see more commands"
)

var (
	rootCmd = &cobra.Command{
		Use:           "go-layout",
		Short:         "go-layout",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `Gin framework is used as the core of this project to build a scaffold,
based on the project can be quickly completed business development, out of the box 📦`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := bootstrapx.InitializeConfig(configPath); err != nil {
				return err
			}
			bootstrapx.InitializeTimezone()
			bootstrapx.InitializeLogger()
			if err := queue.InitPublisher(config.GetConfig()); err != nil {
				log.Warn("Queue publisher initialization failed", zap.Error(err))
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s\n", welcomeMessage)
		},
	}
	configPath string
)

func init() {
	runtime.RegisterConfigReloadHandlers()
	registerFlags()
	registerCommands()
}

// registerFlags 注册命令行标志
func registerFlags() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "The absolute path of the configuration file")
}

// registerCommands 注册子命令
func registerCommands() {
	rootCmd.AddCommand(version.Cmd) // 查看版本: go-layout version
	rootCmd.AddCommand(service.Cmd) // 启动服务: go-layout service
	rootCmd.AddCommand(command.Cmd) // 运行命令: go-layout command demo / go-layout command init api-route
	rootCmd.AddCommand(cron.Cmd)    // 启动计划任务: go-layout cron
	rootCmd.AddCommand(worker.Cmd)  // 启动异步任务 worker: go-layout worker
}

// Execute 执行命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if log.Logger != nil {
			log.Logger.Error("Command execution failed", zap.Error(err))
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
