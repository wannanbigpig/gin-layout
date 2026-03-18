package demo

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Cmd = &cobra.Command{
		Use:     "demo",
		Short:   "这是一个demo",
		Example: "go-layout command demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			demo()
			return nil
		},
	}
	test string
)

func init() {
	Cmd.Flags().StringVarP(&test, "test", "t", "test", "测试接收参数")
}

func demo() {
	fmt.Println("hello console!", test)
}
