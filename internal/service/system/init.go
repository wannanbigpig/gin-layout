package system

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
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
	if err := validator.InitValidatorTrans("zh"); err != nil {
		return fmt.Errorf("初始化验证器失败: %w", err)
	}

	engine := routers.SetRouters()
	apiMap := routers.CollectAdminRouteMeta()

	// 构建API数据
	apiData := s.buildApiData(engine, apiMap)

	// 保存API数据
	if err := s.saveApiData(apiData); err != nil {
		return fmt.Errorf("保存API数据失败: %w", err)
	}
	if err := access.NewMenuAPIDefaultsService().Sync(); err != nil {
		return fmt.Errorf("同步默认菜单接口关系失败: %w", err)
	}

	return access.NewSystemDefaultsService().Ensure()
}

// RebuildUserPermissions 按数据库关系全量重建用户最终 API 权限。
func (s *InitService) RebuildUserPermissions() error {
	// 检查数据库连接
	if err := s.checkDatabaseConnection(); err != nil {
		return err
	}

	// 在方案A中，菜单-API 关系以数据库关系表为准，这里改为全量重建用户最终 API 权限
	if err := access.NewMenuAPIDefaultsService().Sync(); err != nil {
		return err
	}
	if err := access.NewSystemDefaultsService().Ensure(); err != nil {
		return err
	}
	return access.NewUserPermissionSyncService().SyncAllUsers()
}

// buildApiData 构建API数据
func (s *InitService) buildApiData(engine *gin.Engine, apiMap routers.RouteMetaMap) []map[string]any {
	date := time.Now().Format(time.DateTime)
	apiData := make([]map[string]any, 0, len(engine.Routes()))

	for _, route := range engine.Routes() {
		apiInfo := s.extractApiInfo(route, apiMap, date)
		apiData = append(apiData, apiInfo)
	}

	return apiData
}

// extractApiInfo 提取API信息
func (s *InitService) extractApiInfo(route gin.RouteInfo, apiMap routers.RouteMetaMap, date string) map[string]any {
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
	if err := apiModel.InitRegisters(apiData, date); err != nil {
		return err
	}
	return access.NewApiRouteCacheService().RefreshCache()
}

// checkDatabaseConnection 检查数据库连接
func (s *InitService) checkDatabaseConnection() error {
	db := data.MysqlDB()
	if db == nil {
		return fmt.Errorf("数据库连接未初始化，请检查配置")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	return nil
}
