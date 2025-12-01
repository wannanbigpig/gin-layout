package system

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
)

// InitService 初始化服务
type InitService struct{}

// NewInitService 创建初始化服务实例
func NewInitService() *InitService {
	return &InitService{}
}

// InitApiRoutes 初始化API路由
func (s *InitService) InitApiRoutes() error {
	// 检查数据库连接
	if err := s.checkDatabaseConnection(); err != nil {
		return err
	}

	// 初始化验证器
	validator.InitValidatorTrans("zh")

	// 设置路由（需要获取路由信息）
	engine, apiMap := routers.SetRouters(true)

	// 构建API数据
	apiData := s.buildApiData(engine, apiMap)

	// 保存API数据
	if err := s.saveApiData(apiData); err != nil {
		return fmt.Errorf("保存API数据失败: %w", err)
	}

	return nil
}

// InitMenuApiMap 初始化菜单-API映射
func (s *InitService) InitMenuApiMap() error {
	// 检查数据库连接
	if err := s.checkDatabaseConnection(); err != nil {
		return err
	}

	// 执行初始化
	return s.buildMenuApiMap()
}

// checkDatabaseConnection 检查数据库连接
func (s *InitService) checkDatabaseConnection() error {
	db := data.MysqlDB()
	if db == nil {
		return fmt.Errorf("数据库连接未初始化，请检查配置")
	}

	// 测试数据库连接
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	return nil
}

// buildApiData 构建API数据
func (s *InitService) buildApiData(engine *gin.Engine, apiMap routers.ApiMap) []map[string]any {
	date := time.Now().Format(time.DateTime)
	apiData := make([]map[string]any, 0, len(engine.Routes()))

	for _, route := range engine.Routes() {
		apiInfo := s.extractApiInfo(route, apiMap, date)
		apiData = append(apiData, apiInfo)
	}

	return apiData
}

// extractApiInfo 提取API信息
func (s *InitService) extractApiInfo(route gin.RouteInfo, apiMap routers.ApiMap, date string) map[string]any {
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
		"func":         s.extractHandlerName(route.Handler),
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
func (s *InitService) extractHandlerName(handler string) string {
	parts := strings.Split(handler, ".")
	if len(parts) == 0 {
		return handler
	}
	handlerName := parts[len(parts)-1]
	// 移除方法接收器的后缀 "-fm"
	return strings.TrimSuffix(handlerName, "-fm")
}

// saveApiData 保存API数据到数据库
func (s *InitService) saveApiData(apiData []map[string]any) error {
	apiModel := model.NewApi()
	date := time.Now().Format(time.DateTime)
	return apiModel.InitRegisters(apiData, date)
}

// buildMenuApiMap 构建菜单API关联表，根据casbin_rule表自动生成关联
func (s *InitService) buildMenuApiMap() error {
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
		log.Logger.Error("构建菜单API映射失败", zap.Error(err))
		return err
	}

	return nil
}
