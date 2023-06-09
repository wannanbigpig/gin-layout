package command

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/console/demo"
	"github.com/wannanbigpig/gin-layout/internal/routers"
)

var (
	Cmd = &cobra.Command{
		Use:     "command",
		Short:   "The control head runs the command",
		Example: "go-layout command demo",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// 初始化数据库
			data.InitData()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}
	host = "0.0.0.0"
	port = 9001
)

func init() {
	Cmd.AddCommand(demo.DemoCmd)
}

func run() error {
	r := routers.SetRouters()

	err := r.Run(fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	return nil
}
