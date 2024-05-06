package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"github.com/wannanbigpig/gin-layout/internal/validator"
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
	engine := routers.SetRouters()
	if setRoute {
		RegisterRoutes(engine)
		return nil
	}
	err := engine.Run(fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	return nil
}

func RegisterRoutes(engine *gin.Engine) {
	r := engine.Routes()
	apiModel := model.NewPermission()
	var apiData []map[string]any
	date := time.Now()
	for _, v := range r {
		name := v.Path
		desc := ""
		isAuth := 1
		routerMap, ok := routers.AdminRouteMap[v.Path]
		if ok {
			name = routerMap.Name
			desc = routerMap.Desc
			isAuth = routerMap.IsAuth
		}
		apiData = append(apiData, map[string]any{
			"name":       name,
			"desc":       desc,
			"route":      v.Path,
			"method":     v.Method,
			"func":       extractHandler(v.Handler),
			"func_path":  v.Handler,
			"is_auth":    isAuth,
			"sort":       100,
			"created_at": date,
			"updated_at": date,
		})
	}
	err := apiModel.Registers(apiData)
	if err != nil {
		panic(err)
	}
}

func extractHandler(handler string) string {
	// 使用正则表达式提取handler字段中的包名、接收器类型和方法名
	parts := strings.Split(handler, ".")
	handlerName := parts[len(parts)-1]
	return strings.TrimSuffix(handlerName, "-fm")
}
