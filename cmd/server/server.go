package server

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

const (
	defaultHost      = "0.0.0.0"
	defaultPort      = 9001
	defaultSort      = 100
	defaultIsAuth    = 0
	defaultGroupCode = "other"

	// 确认提示信息
	confirmPrompt           = "This command is used to obtain the defined API in the system and store it in the a_api table. Are you sure to perform the operation? [Y/N]: "
	confirmMenuApiMapPrompt = "This command is used to initialize the menu-API mapping table from casbin_rule table. Are you sure to perform the operation? [Y/N]: "
	confirmYes              = "y"

	// 消息提示
	msgUserNotConfirmed       = "User did not confirm. Aborting."
	msgUserConfirmed          = "User confirmed. Proceeding..."
	msgProcessingComplete     = "Processing complete."
	msgFailedToSaveRoute      = "Failed to save the initial route data to the routing table."
	msgMenuApiMapComplete     = "Menu-API mapping initialization complete."
	msgFailedToInitMenuApiMap = "Failed to initialize menu-API mapping."
)

var (
	Cmd = &cobra.Command{
		Use:     "server",
		Short:   "Start API server",
		Example: "go-layout server -c config.yml",
		PreRun: func(cmd *cobra.Command, args []string) {
			initializeServer()
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
	registerFlags()
}

// registerFlags 注册命令行标志
func registerFlags() {
	Cmd.Flags().StringVarP(&host, "host", "H", defaultHost, "监听服务器地址")
	Cmd.Flags().IntVarP(&port, "port", "P", defaultPort, "监听服务器端口")
	Cmd.Flags().BoolVarP(&setRoute, "set-route", "S", false, "设置数据库API路由表")
}

// initializeServer 初始化服务器
func initializeServer() {
	// 初始化数据库
	data.InitData()

	// 初始化验证器
	validator.InitValidatorTrans("zh")
}

// run 运行服务器
func run() error {
	engine, apiMap := routers.SetRouters(setRoute)

	// 如果需要设置路由，执行路由注册
	if setRoute {
		registerRoutes(engine, apiMap)
	}

	address := fmt.Sprintf("%s:%d", host, port)
	return engine.Run(address)
}

// registerRoutes 注册路由到数据库
func registerRoutes(e *gin.Engine, apiMap routers.ApiMap) {
	if !confirmRouteRegistration() {
		return
	}

	apiData := buildApiData(e, apiMap)
	if err := saveApiData(apiData); err != nil {
		log.Logger.Error(msgFailedToSaveRoute, zap.Error(err))
		fmt.Println(msgFailedToSaveRoute)
		os.Exit(1)
	}

	fmt.Println(msgProcessingComplete)

	// 询问是否初始化菜单API关联表
	if !confirmMenuApiMapInit() {
		return
	}

	if err := initMenuApiMap(); err != nil {
		log.Logger.Error(msgFailedToInitMenuApiMap, zap.Error(err))
		fmt.Println(msgFailedToInitMenuApiMap)
		os.Exit(1)
	}

	fmt.Println(msgMenuApiMapComplete)
}

// confirmRouteRegistration 确认是否执行路由注册操作
func confirmRouteRegistration() bool {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(confirmPrompt)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			log.Logger.Error("Failed to read user input", zap.Error(err))
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		return false
	}

	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if input != confirmYes {
		fmt.Println(msgUserNotConfirmed)
		return false
	}

	fmt.Println(msgUserConfirmed)
	return true
}

// buildApiData 构建API数据
func buildApiData(e *gin.Engine, apiMap routers.ApiMap) []map[string]any {
	date := time.Now().Format(time.DateTime)
	apiData := make([]map[string]any, 0, len(e.Routes()))

	for _, route := range e.Routes() {
		apiInfo := extractApiInfo(route, apiMap, date)
		apiData = append(apiData, apiInfo)
	}

	return apiData
}

// extractApiInfo 提取API信息
func extractApiInfo(route gin.RouteInfo, apiMap routers.ApiMap, date string) map[string]any {
	code := utils.MD5(route.Method + "_" + route.Path)
	name := route.Path
	isAuth := defaultIsAuth
	desc := ""
	groupCode := defaultGroupCode

	// 从 apiMap 中获取路由元信息
	if val, ok := apiMap[code]; ok {
		name = val.Title
		isAuth = int(val.Auth)
		desc = val.Desc
		groupCode = val.GroupCode
	}

	return map[string]any{
		"code":         code,
		"name":         name,
		"route":        route.Path,
		"method":       route.Method,
		"func":         extractHandlerName(route.Handler),
		"func_path":    route.Handler,
		"is_auth":      isAuth,
		"description":  desc,
		"sort":         defaultSort,
		"is_effective": 1,
		"created_at":   date,
		"updated_at":   date,
		"group_code":   groupCode,
	}
}

// extractHandlerName 提取处理器名称
func extractHandlerName(handler string) string {
	parts := strings.Split(handler, ".")
	if len(parts) == 0 {
		return handler
	}
	handlerName := parts[len(parts)-1]
	// 移除方法接收器的后缀 "-fm"
	return strings.TrimSuffix(handlerName, "-fm")
}

// saveApiData 保存API数据到数据库
func saveApiData(apiData []map[string]any) error {
	apiModel := model.NewApi()
	date := time.Now().Format(time.DateTime)
	return apiModel.InitRegisters(apiData, date)
}

// confirmMenuApiMapInit 确认是否执行菜单API关联表初始化操作
func confirmMenuApiMapInit() bool {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(confirmMenuApiMapPrompt)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			log.Logger.Error("Failed to read user input", zap.Error(err))
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		return false
	}

	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if input != confirmYes {
		fmt.Println(msgUserNotConfirmed)
		return false
	}

	fmt.Println(msgUserConfirmed)
	return true
}

// initMenuApiMap 初始化菜单API关联表，根据casbin_rule表自动生成关联
func initMenuApiMap() error {
	db := data.MysqlDB()

	// 执行 SQL：从 casbin_rule 表中提取菜单ID和API的route+method来关联
	sql := `INSERT INTO a_menu_api_map (menu_id, api_id, created_at, updated_at)
			SELECT 
				CAST(SUBSTRING_INDEX(c.v0, ':', -1) AS UNSIGNED) AS menu_id,
				a.id AS api_id,
				NOW() AS created_at,
				NOW() AS updated_at
			FROM casbin_rule c
			INNER JOIN a_api a ON a.route = c.v1 AND a.method = c.v2 AND a.deleted_at = 0
			INNER JOIN a_menu m ON m.id = CAST(SUBSTRING_INDEX(c.v0, ':', -1) AS UNSIGNED) AND m.deleted_at = 0
			WHERE c.ptype = 'p' 
			  AND c.v0 LIKE 'menu:%'
			  AND c.v1 != ''
			  AND c.v2 != ''
			ON DUPLICATE KEY UPDATE updated_at = NOW()`

	if err := db.Exec(sql).Error; err != nil {
		return err
	}

	return nil
}
