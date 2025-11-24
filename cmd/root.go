package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/cmd/command"
	"github.com/wannanbigpig/gin-layout/cmd/cron"
	"github.com/wannanbigpig/gin-layout/cmd/server"
	"github.com/wannanbigpig/gin-layout/cmd/version"
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

const (
	welcomeMessage       = "Welcome to go-layout. Use -h to see more commands"
	errorLoadingLocation = "Error loading location: %v"
)

var (
	rootCmd = &cobra.Command{
		Use:          "go-layout",
		Short:        "go-layout",
		SilenceUsage: true,
		Long: `Gin framework is used as the core of this project to build a scaffold,
based on the project can be quickly completed business development, out of the box 📦`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			initializeConfig()
			initializeTimezone()
			initializeLogger()
		},
		Run: func(cmd *cobra.Command, args []string) {
			if printVersion {
				fmt.Println(global.Version)
				return
			}
			fmt.Printf("%s\n", welcomeMessage)
		},
	}
	configPath   string
	printVersion bool
)

func init() {
	registerFlags()
	registerCommands()
}

// registerFlags 注册命令行标志
func registerFlags() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "The absolute path of the configuration file")
	rootCmd.Flags().BoolVarP(&printVersion, "version", "v", false, "Get version info")
}

// registerCommands 注册子命令
func registerCommands() {
	rootCmd.AddCommand(version.Cmd) // 查看版本: go-layout version
	rootCmd.AddCommand(server.Cmd)  // 启动服务: go-layout server
	rootCmd.AddCommand(command.Cmd) // 启动单次运行脚本: go-layout command demo
	rootCmd.AddCommand(cron.Cmd)    // 启动计划任务: go-layout cron
}

// initializeConfig 初始化配置
func initializeConfig() {
	config.InitConfig(configPath)
}

// initializeTimezone 初始化时区
func initializeTimezone() {
	if config.Config.Timezone == nil {
		return
	}

	location, err := time.LoadLocation(*config.Config.Timezone)
	if err != nil {
		log.Logger.Error(fmt.Sprintf(errorLoadingLocation, err), zap.Error(err))
		fmt.Printf(errorLoadingLocation+"\n", err)
		return
	}
	time.Local = location
}

// initializeLogger 初始化日志
func initializeLogger() {
	log.InitLogger()
}

// Execute 执行命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Logger.Error("Command execution failed", zap.Error(err))
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
