package server

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

var (
	Cmd = &cobra.Command{
		Use:     "server",
		Short:   "Start API server",
		Example: "go-layout server -c config.yml",
		PreRun: func(cmd *cobra.Command, args []string) {
			// 初始化数据库
			data.InitData()

			// 初始化验证器
			validator.InitValidatorTrans("zh")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}
	host     string
	port     int
	setRoute bool
)

func init() {
	Cmd.Flags().StringVarP(&host, "host", "H", "0.0.0.0", "监听服务器地址")
	Cmd.Flags().IntVarP(&port, "port", "P", 9001, "监听服务器端口")
	Cmd.Flags().BoolVarP(&setRoute, "set-route", "R", false, "设置数据库数据库API路由表")
}

func run() error {
	engine, apiMap := routers.SetRouters(setRoute)
	if setRoute {
		RegisterRoutes(engine, apiMap)
	}
	err := engine.Run(fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	return nil
}

func RegisterRoutes(e *gin.Engine, apiMap routers.ApiMap) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("This command is used to obtain the defined API in the system and store it in the a_api table. Are you sure to perform the operation? [Y/N]: ")

	// 读取用户输入
	if scanner.Scan() {
		input := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if input == "y" {
			fmt.Println("User confirmed. Proceeding...")
			// 在这里执行敏感操作
			apiModel := model.NewApi()
			var apiData []map[string]any
			date := time.Now().Format(time.DateTime)
			for _, v := range e.Routes() {
				name := v.Path
				code := utils.MD5(v.Method + "_" + v.Path)
				desc := ""
				var isAuth uint8 = 1
				groupCode := "other"
				if val, ok := apiMap[code]; ok {
					name = val.Title
					isAuth = val.Auth
					desc = utils.If(val.Desc != "", val.Desc, name)
					groupCode = val.GroupCode
				}
				apiData = append(apiData, map[string]any{
					"code":         code,
					"name":         name,
					"route":        v.Path,
					"method":       v.Method,
					"func":         extractHandler(v.Handler),
					"func_path":    v.Handler,
					"is_auth":      isAuth,
					"desc":         desc,
					"sort":         100,
					"is_effective": 1,
					"created_at":   date,
					"updated_at":   date,
					"group_code":   groupCode,
				})
			}
			err := apiModel.InitRegisters(apiData, date)
			if err != nil {
				fmt.Println("Failed to save the initial route data to the routing table. ")
				panic(err)
			}

			fmt.Println("Processing complete.")
		} else {
			fmt.Println("User did not confirm. Aborting.")
		}
	} else if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}

	return
}

func extractHandler(handler string) string {
	// 使用正则表达式提取handler字段中的包名、接收器类型和方法名
	parts := strings.Split(handler, ".")
	handlerName := parts[len(parts)-1]
	return strings.TrimSuffix(handlerName, "-fm")
}
