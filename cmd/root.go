package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/cmd/command"
	"github.com/wannanbigpig/gin-layout/cmd/cron"
	"github.com/wannanbigpig/gin-layout/cmd/server"
	"github.com/wannanbigpig/gin-layout/cmd/version"
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

var (
	rootCmd = &cobra.Command{
		Use:          "go-layout",
		Short:        "go-layout",
		SilenceUsage: true,
		Long: `Gin framework is used as the core of this project to build a scaffold,
based on the project can be quickly completed business development, out of the box 📦`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// 1、初始化配置
			config.InitConfig(configPath)
			// 2、初始化zap日志
			logger.InitLogger()
		},
		Run: func(cmd *cobra.Command, args []string) {
			if printVersion {
				fmt.Println(global.Version)
				return
			}

			fmt.Printf("%s\n", "Welcome to go-layout. Use -h to see more commands")
		},
	}
	configPath   string
	printVersion bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "The absolute path of the configuration file")
	rootCmd.Flags().BoolVarP(&printVersion, "version", "v", false, "GetUserInfo version info")
	// 查看版本 go-layout version
	rootCmd.AddCommand(version.Cmd)
	// 启动服务 go-layout server
	rootCmd.AddCommand(server.Cmd)
	// 启动单词运行脚本 go-layout command demo
	rootCmd.AddCommand(command.Cmd)
	// 启动计划任务
	rootCmd.AddCommand(cron.Cmd)
}

// Execute : apply commands
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}
