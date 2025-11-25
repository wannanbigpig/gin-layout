package command

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/console/demo"
	"github.com/wannanbigpig/gin-layout/internal/routers"
)

const (
	defaultHost = "0.0.0.0"
	defaultPort = 9001
)

var (
	Cmd = &cobra.Command{
		Use:     "command",
		Short:   "The control head runs the command",
		Example: "go-layout command demo",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			data.InitData()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}
	host string
	port int
)

func init() {
	registerSubCommands()
	registerFlags()
}

// registerSubCommands 注册子命令
func registerSubCommands() {
	Cmd.AddCommand(demo.DemoCmd)
}

// registerFlags 注册命令行标志
func registerFlags() {
	Cmd.Flags().StringVarP(&host, "host", "H", defaultHost, "监听服务器地址")
	Cmd.Flags().IntVarP(&port, "port", "P", defaultPort, "监听服务器端口")
}

// run 运行命令
func run() error {
	engine, _ := routers.SetRouters(false)
	address := fmt.Sprintf("%s:%d", host, port)
	return engine.Run(address)
}
