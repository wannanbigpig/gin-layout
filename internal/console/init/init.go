package init

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
	defaultSort      = 100
	defaultIsAuth    = 0
	defaultGroupCode = "other"

	msgProcessingComplete     = "Processing complete."
	msgFailedToSaveRoute      = "Failed to save the initial route data to the routing table."
	msgMenuApiMapComplete     = "Menu-API mapping initialization complete."
	msgFailedToInitMenuApiMap = "Failed to initialize menu-API mapping."
)

var (
	ApiRouteCmd = &cobra.Command{
		Use:   "api-route",
		Short: "Initialize API route table",
		Long:  "This command scans all defined API routes in the system and stores them in the a_api table for permission management and API documentation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitApiRoute()
		},
	}

	MenuApiMapCmd = &cobra.Command{
		Use:   "menu-api-map",
		Short: "Initialize menu-API mapping table from casbin_rule table",
		Long:  "This command initializes the a_menu_api_map table by extracting menu-API relationships from the casbin_rule table.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitMenuApiMap()
		},
	}
)

func init() {
	// 可以在这里注册其他初始化相关的子命令
}

// runInitApiRoute 执行API路由表初始化
func runInitApiRoute() error {
	// 用户确认
	if !confirmOperation("This command is used to obtain the defined API in the system and store it in the a_api table. Are you sure to perform the operation? [Y/N]: ") {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// 检查数据库连接是否正常
	db := data.MysqlDB()
	if db == nil {
		errMsg := "数据库连接未初始化，请检查配置"
		log.Logger.Error(errMsg)
		fmt.Println(errMsg)
		return fmt.Errorf("数据库连接未初始化，请检查配置")
	}

	// 测试数据库连接
	sqlDB, err := db.DB()
	if err != nil {
		log.Logger.Error("获取数据库连接失败", zap.Error(err))
		fmt.Printf("获取数据库连接失败: %v\n", err)
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Logger.Error("数据库连接测试失败", zap.Error(err))
		fmt.Printf("数据库连接测试失败: %v\n", err)
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 初始化验证器
	validator.InitValidatorTrans("zh")

	// 设置路由（需要获取路由信息）
	engine, apiMap := routers.SetRouters(true)

	// 构建API数据
	apiData := buildApiData(engine, apiMap)

	// 保存API数据
	if err := saveApiData(apiData); err != nil {
		log.Logger.Error(msgFailedToSaveRoute, zap.Error(err))
		fmt.Println(msgFailedToSaveRoute)
		return err
	}

	fmt.Println(msgProcessingComplete)
	return nil
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

// runInitMenuApiMap 执行菜单API映射初始化
func runInitMenuApiMap() error {
	// 用户确认
	if !confirmOperation("This command is used to initialize the menu-API mapping table from casbin_rule table. Are you sure to perform the operation? [Y/N]: ") {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// 检查数据库连接是否正常
	db := data.MysqlDB()
	if db == nil {
		errMsg := "数据库连接未初始化，请检查配置"
		log.Logger.Error(errMsg)
		fmt.Println(errMsg)
		return fmt.Errorf("数据库连接未初始化，请检查配置")
	}

	// 测试数据库连接
	sqlDB, err := db.DB()
	if err != nil {
		log.Logger.Error("获取数据库连接失败", zap.Error(err))
		fmt.Printf("获取数据库连接失败: %v\n", err)
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Logger.Error("数据库连接测试失败", zap.Error(err))
		fmt.Printf("数据库连接测试失败: %v\n", err)
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 执行初始化
	if err := buildMenuApiMap(); err != nil {
		log.Logger.Error(msgFailedToInitMenuApiMap, zap.Error(err))
		fmt.Println(msgFailedToInitMenuApiMap)
		return err
	}

	fmt.Println(msgMenuApiMapComplete)
	return nil
}

// confirmOperation 确认操作，返回用户是否确认
func confirmOperation(prompt string) bool {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(prompt)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			log.Logger.Error("Failed to read user input", zap.Error(err))
			_, err := fmt.Fprintln(os.Stderr, "reading standard input:", err)
			if err != nil {
				return false
			}
		}
		return false
	}

	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return input == "y" || input == "yes"
}

// buildMenuApiMap 构建菜单API关联表，根据casbin_rule表自动生成关联
func buildMenuApiMap() error {
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
