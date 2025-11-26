package command

import (
	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/console/demo"
	initconsole "github.com/wannanbigpig/gin-layout/internal/console/init"
)

var (
	Cmd = &cobra.Command{
		Use:     "command",
		Short:   "The control head runs the command",
		Example: "go-layout command demo",
		PreRun: func(cmd *cobra.Command, args []string) {
			// 初始化数据库
			data.InitData()
		},
	}
)

func init() {
	registerSubCommands()
}

// registerSubCommands 注册子命令
func registerSubCommands() {
	// 一次性运行脚本
	Cmd.AddCommand(demo.Cmd)
	Cmd.AddCommand(initconsole.ApiRouteCmd)   // 初始化API路由表: go-layout command api-route
	Cmd.AddCommand(initconsole.MenuApiMapCmd) // 初始化菜单-API映射: go-layout command menu-api-map
}
