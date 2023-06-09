package version

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/wannanbigpig/gin-layout/internal/global"
)

var (
	Cmd = &cobra.Command{
		Use:     "version",
		Short:   "GetUserInfo version info",
		Example: "go-layout version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(global.Version)
		},
	}
)
