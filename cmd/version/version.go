package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

var (
	// Cmd 版本信息命令
	Cmd = &cobra.Command{
		Use:     "version",
		Short:   "Get version info",
		Example: "go-layout version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(global.Version)
			return nil
		},
	}
)
