package version

import (
	"encoding/json"
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
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
					"version": global.Version, "commit": global.BuildCommit,
					"builtAt": global.BuildTime, "dirty": global.BuildDirty,
				})
			}
			fmt.Fprintln(cmd.OutOrStdout(), global.Version)
			return nil
		},
	}
)

func init() {
	Cmd.Flags().Bool("json", false, "Output application version and build identity as JSON")
}
