package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/data"
	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

func init() {
	_ = taskcron.RegisterHandler(taskcron.HandlerCronResetSystemData, func(ctx context.Context, payload map[string]any) error {
		_ = ctx
		_ = payload
		return ReinitializeSystemData()
	})
}

const (
	defaultSort = 100
)

// InitApiRoutes 初始化API路由
func InitApiRoutes() error {
	// 检查数据库连接
	if err := checkDatabaseConnection(); err != nil {
		return err
	}

	// 初始化验证器
	if err := validator.InitValidatorTrans("zh"); err != nil {
		return fmt.Errorf("初始化验证器失败: %w", err)
	}

	routeTree := routers.AppRouteTree()
	catalog, err := routers.CompileRouteTree(routeTree)
	if err != nil {
		return fmt.Errorf("编译路由失败: %w", err)
	}
	engine, err := routers.SetRoutersWithCatalog(catalog)
	if err != nil {
		return fmt.Errorf("初始化路由失败: %w", err)
	}

	// 构建API数据
	apiData, err := buildApiData(engine.Routes(), catalog.MetaMap())
	if err != nil {
		return fmt.Errorf("构建 API 路由数据失败: %w", err)
	}

	// 保存API数据
	if err := saveApiData(apiData); err != nil {
		return fmt.Errorf("保存API数据失败: %w", err)
	}
	if err := access.NewPermissionSyncCoordinator().SyncAll(); err != nil {
		return fmt.Errorf("同步权限策略闭环失败: %w", err)
	}
	return nil
}

// RebuildUserPermissions 按数据库关系全量重建用户最终 API 权限。
func RebuildUserPermissions() error {
	// 检查数据库连接
	if err := checkDatabaseConnection(); err != nil {
		return err
	}

	// 在方案A中，菜单-API 关系以数据库关系表为准，这里改为全量重建用户最终 API 权限
	return access.NewPermissionSyncCoordinator().SyncAll()
}

// buildApiData 构建API数据
func buildApiData(routes []gin.RouteInfo, apiMap routers.RouteMetaMap) ([]map[string]any, error) {
	date := time.Now().Format(time.DateTime)
	apiData := make([]map[string]any, 0, len(routes))

	for _, route := range routes {
		code := utils.MD5(route.Method + "_" + route.Path)
		meta, ok := apiMap[code]
		if !ok {
			return nil, fmt.Errorf("missing route metadata for %s %s", route.Method, route.Path)
		}

		apiData = append(apiData, map[string]any{
			"code":             code,
			"name":             meta.Title,
			"name_i18n":        i18n.MarshalLocaleMap(meta.TitleI18n),
			"route":            route.Path,
			"method":           route.Method,
			"func":             extractHandlerName(route.Handler),
			"func_path":        route.Handler,
			"is_auth":          int(meta.Auth),
			"description":      meta.Desc,
			"description_i18n": i18n.MarshalLocaleMap(meta.DescI18n),
			"sort":             defaultSort,
			"is_effective":     1,
			"created_at":       date,
			"updated_at":       date,
			"group_code":       meta.GroupCode,
		})
	}

	return apiData, nil
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
	if err := apiModel.InitRegisters(apiData, date); err != nil {
		return err
	}
	return access.NewApiRouteCacheService().RefreshCache()
}

// checkDatabaseConnection 检查数据库连接
func checkDatabaseConnection() error {
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
