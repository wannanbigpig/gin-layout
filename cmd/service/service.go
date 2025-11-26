package service

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"github.com/wannanbigpig/gin-layout/internal/validator"
)

const (
	defaultHost = "0.0.0.0"
	defaultPort = 9001
)

var (
	Cmd = &cobra.Command{
		Use:     "service",
		Short:   "Start API service",
		Example: "go-layout service -c config.yml",
		PreRun: func(cmd *cobra.Command, args []string) {
			initializeService()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}
	host string
	port int
)

func init() {
	registerFlags()
}

// registerFlags 注册命令行标志
func registerFlags() {
	Cmd.Flags().StringVarP(&host, "host", "H", defaultHost, "监听服务器地址")
	Cmd.Flags().IntVarP(&port, "port", "P", defaultPort, "监听服务器端口")
}

// initializeService 初始化服务器
func initializeService() {
	// 初始化数据库
	data.InitData()

	// 初始化验证器
	validator.InitValidatorTrans("zh")
}

// run 运行服务器
func run() error {
	engine, _ := routers.SetRouters(false)
	address := fmt.Sprintf("%s:%d", host, port)
	return engine.Run(address)
}
