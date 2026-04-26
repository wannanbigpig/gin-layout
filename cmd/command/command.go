package command

import (
	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/cmd/bootstrapx"
	"github.com/wannanbigpig/gin-layout/internal/console/demo"
	initconsole "github.com/wannanbigpig/gin-layout/internal/console/init"
	migrateconsole "github.com/wannanbigpig/gin-layout/internal/console/migrate"
	"github.com/wannanbigpig/gin-layout/internal/console/system_init"
	taskconsole "github.com/wannanbigpig/gin-layout/internal/console/task"
)

var (
	Cmd = &cobra.Command{
		Use:     "command",
		Short:   "The control head runs the command",
		Example: "go-layout command demo",
	}
)

func init() {
	registerSubCommands()
}

// registerSubCommands 注册子命令
func registerSubCommands() {
	// 一次性运行脚本
	Cmd.AddCommand(demo.Cmd)
	Cmd.AddCommand(bootstrapx.WrapCommand(initconsole.ApiRouteCmd, bootstrapx.Requirements{Data: true}))               // 初始化API路由表: go-layout command api-route
	Cmd.AddCommand(bootstrapx.WrapCommand(initconsole.RebuildUserPermissionsCmd, bootstrapx.Requirements{Data: true})) // 重建用户最终 API 权限: go-layout command rebuild-user-permissions
	Cmd.AddCommand(bootstrapx.WrapCommand(system_init.InitSystemCmd, bootstrapx.Requirements{Data: true}))             // 初始化系统: go-layout command init-system
	Cmd.AddCommand(migrateconsole.Cmd)                                                                                 // 迁移管理: go-layout command migrate up / down / create / check
	Cmd.AddCommand(bootstrapx.WrapCommand(taskconsole.Cmd, bootstrapx.Requirements{Data: true}))                       // 任务扫描: go-layout command task scan-async
}
