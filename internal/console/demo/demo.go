package demo

import (
	"fmt"
	"github.com/spf13/cobra"
)

var (
	DemoCmd = &cobra.Command{
		Use:     "demo",
		Short:   "这是一个demo",
		Example: "go-layout command demo",
		Run: func(cmd *cobra.Command, args []string) {
			demo()
		},
	}
	test string
)

func init() {
	DemoCmd.Flags().StringVarP(&test, "test", "t", "test", "测试接收参数")
}

func demo() {
	fmt.Println("hello console!", test)
}
