package access

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

type defaultMenuAPIBinding struct {
	// MenuCode 菜单编码。
	MenuCode string
	// Route 绑定的接口路由。
	Route string
	// Method 绑定的 HTTP 方法。
	Method string
}

var builtInDefaultMenuAPIBindings = [...]defaultMenuAPIBinding{
	{MenuCode: "adminUser:update", Route: "/admin/v1/admin-user/update", Method: "POST"},
	{MenuCode: "adminUser:add", Route: "/admin/v1/admin-user/create", Method: "POST"},
	{MenuCode: "adminUser:bindRole", Route: "/admin/v1/admin-user/bind-role", Method: "POST"},
	{MenuCode: "adminUser:bindRole", Route: "/admin/v1/admin-user/detail", Method: "GET"},
	{MenuCode: "adminUser:bindRole", Route: "/admin/v1/role/list", Method: "GET"},
	{MenuCode: "adminUser:delete", Route: "/admin/v1/admin-user/delete", Method: "POST"},
	{MenuCode: "menu:add", Route: "/admin/v1/menu/create", Method: "POST"},
	{MenuCode: "menu:add", Route: "/admin/v1/permission/list", Method: "GET"},
	{MenuCode: "menu:addChild", Route: "/admin/v1/menu/create", Method: "POST"},
	{MenuCode: "menu:addChild", Route: "/admin/v1/permission/list", Method: "GET"},
	{MenuCode: "menu:update", Route: "/admin/v1/menu/detail", Method: "GET"},
	{MenuCode: "menu:update", Route: "/admin/v1/menu/update", Method: "POST"},
	{MenuCode: "menu:update", Route: "/admin/v1/permission/list", Method: "GET"},
	{MenuCode: "menu:delete", Route: "/admin/v1/menu/delete", Method: "POST"},
	{MenuCode: "role:add", Route: "/admin/v1/menu/list", Method: "GET"},
	{MenuCode: "role:add", Route: "/admin/v1/role/create", Method: "POST"},
	{MenuCode: "role:update", Route: "/admin/v1/menu/list", Method: "GET"},
	{MenuCode: "role:update", Route: "/admin/v1/role/detail", Method: "GET"},
	{MenuCode: "role:update", Route: "/admin/v1/role/update", Method: "POST"},
	{MenuCode: "role:delete", Route: "/admin/v1/role/delete", Method: "POST"},
	{MenuCode: "department:add", Route: "/admin/v1/department/create", Method: "POST"},
	{MenuCode: "department:addChild", Route: "/admin/v1/department/create", Method: "POST"},
	{MenuCode: "department:update", Route: "/admin/v1/department/update", Method: "POST"},
	{MenuCode: "department:bindRole", Route: "/admin/v1/department/bind-role", Method: "POST"},
	{MenuCode: "department:bindRole", Route: "/admin/v1/department/detail", Method: "GET"},
	{MenuCode: "department:bindRole", Route: "/admin/v1/role/list", Method: "GET"},
	{MenuCode: "department:delete", Route: "/admin/v1/department/delete", Method: "POST"},
	{MenuCode: "api:update", Route: "/admin/v1/permission/update", Method: "POST"},
	{MenuCode: "role:addChild", Route: "/admin/v1/role/create", Method: "POST"},
	{MenuCode: "adminLoginLog:detail", Route: "/admin/v1/log/login/detail", Method: "GET"},
	{MenuCode: "requestLog:detail", Route: "/admin/v1/log/request/detail", Method: "GET"},
	{MenuCode: "adminUser:list", Route: "/admin/v1/department/list", Method: "GET"},
	{MenuCode: "adminUser:list", Route: "/admin/v1/admin-user/list", Method: "GET"},
	{MenuCode: "department:list", Route: "/admin/v1/department/list", Method: "GET"},
	{MenuCode: "role:list", Route: "/admin/v1/role/list", Method: "GET"},
	{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: "GET"},
	{MenuCode: "api:list", Route: "/admin/v1/permission/list", Method: "GET"},
	{MenuCode: "adminLoginLog:list", Route: "/admin/v1/log/login/list", Method: "GET"},
	{MenuCode: "requestLog:list", Route: "/admin/v1/log/request/list", Method: "GET"},
	{MenuCode: "requestLog:export", Route: "/admin/v1/log/request/export", Method: "GET"},
	{MenuCode: "requestLog:maskConfig", Route: "/admin/v1/log/request/mask-config", Method: "GET"},
	{MenuCode: "requestLog:maskConfig", Route: "/admin/v1/log/request/mask-config", Method: "POST"},
	{MenuCode: "sysConfig:list", Route: "/admin/v1/system/config/list", Method: "GET"},
	{MenuCode: "sysConfig:list", Route: "/admin/v1/system/config/detail", Method: "GET"},
	{MenuCode: "sysConfig:list", Route: "/admin/v1/system/config/value", Method: "GET"},
	{MenuCode: "sysConfig:add", Route: "/admin/v1/system/config/create", Method: "POST"},
	{MenuCode: "sysConfig:update", Route: "/admin/v1/system/config/update", Method: "POST"},
	{MenuCode: "sysConfig:delete", Route: "/admin/v1/system/config/delete", Method: "POST"},
	{MenuCode: "sysConfig:refresh", Route: "/admin/v1/system/config/refresh", Method: "POST"},
	{MenuCode: "sysDict:list", Route: "/admin/v1/system/dict/type/list", Method: "GET"},
	{MenuCode: "sysDict:list", Route: "/admin/v1/system/dict/type/detail", Method: "GET"},
	{MenuCode: "sysDict:list", Route: "/admin/v1/system/dict/item/list", Method: "GET"},
	{MenuCode: "sysDict:list", Route: "/admin/v1/system/dict/options", Method: "GET"},
	{MenuCode: "sysDict:add", Route: "/admin/v1/system/dict/type/create", Method: "POST"},
	{MenuCode: "sysDict:add", Route: "/admin/v1/system/dict/item/create", Method: "POST"},
	{MenuCode: "sysDict:update", Route: "/admin/v1/system/dict/type/update", Method: "POST"},
	{MenuCode: "sysDict:update", Route: "/admin/v1/system/dict/item/update", Method: "POST"},
	{MenuCode: "sysDict:delete", Route: "/admin/v1/system/dict/type/delete", Method: "POST"},
	{MenuCode: "sysDict:delete", Route: "/admin/v1/system/dict/item/delete", Method: "POST"},
	{MenuCode: "task:list", Route: "/admin/v1/task/list", Method: "GET"},
	{MenuCode: "task:list", Route: "/admin/v1/task/run/list", Method: "GET"},
	{MenuCode: "task:detail", Route: "/admin/v1/task/run/detail", Method: "GET"},
	{MenuCode: "task:list", Route: "/admin/v1/task/cron/state", Method: "GET"},
	{MenuCode: "task:trigger", Route: "/admin/v1/task/trigger", Method: "POST"},
	{MenuCode: "task:retry", Route: "/admin/v1/task/run/retry", Method: "POST"},
	{MenuCode: "task:cancel", Route: "/admin/v1/task/run/cancel", Method: "POST"},
}

// MenuAPIDefaultsService 负责初始化默认菜单与接口映射关系。
type MenuAPIDefaultsService struct {
	// bindings 默认菜单与接口绑定配置。
	bindings []defaultMenuAPIBinding
}

// MenuAPIDefaultsServiceDeps 描述 MenuAPIDefaultsService 可注入依赖。
type MenuAPIDefaultsServiceDeps struct {
	// Bindings 自定义默认菜单接口绑定。
	Bindings []defaultMenuAPIBinding
}

// NewMenuAPIDefaultsService 创建默认菜单接口映射服务实例。
func NewMenuAPIDefaultsService() *MenuAPIDefaultsService {
	return NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{})
}

// NewMenuAPIDefaultsServiceWithDeps 创建带依赖注入的默认菜单接口映射服务实例。
func NewMenuAPIDefaultsServiceWithDeps(deps MenuAPIDefaultsServiceDeps) *MenuAPIDefaultsService {
	s := &MenuAPIDefaultsService{}
	if deps.Bindings != nil {
		s.bindings = cloneMenuAPIBindings(deps.Bindings)
	} else {
		s.bindings = defaultMenuAPIBindings()
	}
	return s
}

func defaultMenuAPIBindings() []defaultMenuAPIBinding {
	return cloneMenuAPIBindings(builtInDefaultMenuAPIBindings[:])
}

func cloneMenuAPIBindings(source []defaultMenuAPIBinding) []defaultMenuAPIBinding {
	if len(source) == 0 {
		return nil
	}
	cloned := make([]defaultMenuAPIBinding, len(source))
	copy(cloned, source)
	return cloned
}

// Sync 将默认菜单接口映射写入数据库。
func (s *MenuAPIDefaultsService) Sync(tx ...*gorm.DB) error {
	bindings := s.bindings
	if len(bindings) == 0 {
		return nil
	}

	db, err := defaultMenuAPIDB(FirstTx(tx))
	if err != nil {
		return err
	}

	menuCodes, routes, methods := collectMenuAPIBindingKeys(bindings)
	targets, err := loadDefaultMenuAPITargets(db, menuCodes, routes, methods)
	if err != nil {
		return err
	}

	mappings := buildDefaultMenuAPIMappings(bindings, targets)
	if len(mappings) == 0 {
		return nil
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "menu_id"}, {Name: "api_id"}},
		DoNothing: true,
	}).Create(&mappings).Error
}

func defaultMenuAPIDB(tx *gorm.DB) (*gorm.DB, error) {
	if tx != nil {
		return tx, nil
	}
	return model.NewMenuApiMap().GetDB()
}

func collectMenuAPIBindingKeys(bindings []defaultMenuAPIBinding) (menuCodes []string, routes []string, methods []string) {
	menuCodeSet := make(map[string]struct{}, len(bindings))
	routeSet := make(map[string]struct{}, len(bindings))
	methodSet := make(map[string]struct{}, len(bindings))

	for _, item := range bindings {
		if _, ok := menuCodeSet[item.MenuCode]; !ok {
			menuCodeSet[item.MenuCode] = struct{}{}
			menuCodes = append(menuCodes, item.MenuCode)
		}
		if _, ok := routeSet[item.Route]; !ok {
			routeSet[item.Route] = struct{}{}
			routes = append(routes, item.Route)
		}
		if _, ok := methodSet[item.Method]; !ok {
			methodSet[item.Method] = struct{}{}
			methods = append(methods, item.Method)
		}
	}
	return menuCodes, routes, methods
}

type defaultMenuAPITargets struct {
	menuIDByCode       map[string]uint
	apiIDByRouteMethod map[string]uint
}

func loadDefaultMenuAPITargets(db *gorm.DB, menuCodes []string, routes []string, methods []string) (defaultMenuAPITargets, error) {
	menuModel := model.NewMenu()
	menuModel.SetDB(db)
	menus, err := menuModel.FindIdsByCodes(menuCodes)
	if err != nil {
		return defaultMenuAPITargets{}, err
	}
	menuIDByCode := make(map[string]uint, len(menus))
	for _, menu := range menus {
		menuIDByCode[menu.Code] = menu.ID
	}

	apiModel := model.NewApi()
	apiModel.SetDB(db)
	apis, err := apiModel.FindIdsByRouteAndMethod(routes, methods)
	if err != nil {
		return defaultMenuAPITargets{}, err
	}
	apiIDByRouteMethod := make(map[string]uint, len(apis))
	for _, api := range apis {
		apiIDByRouteMethod[api.Method+":"+api.Route] = api.ID
	}

	return defaultMenuAPITargets{
		menuIDByCode:       menuIDByCode,
		apiIDByRouteMethod: apiIDByRouteMethod,
	}, nil
}

func buildDefaultMenuAPIMappings(bindings []defaultMenuAPIBinding, targets defaultMenuAPITargets) []*model.MenuApiMap {
	mappings := make([]*model.MenuApiMap, 0, len(bindings))
	for _, item := range bindings {
		menuID, ok := targets.menuIDByCode[item.MenuCode]
		if !ok {
			continue
		}
		apiID, ok := targets.apiIDByRouteMethod[item.Method+":"+item.Route]
		if !ok {
			continue
		}
		mappings = append(mappings, &model.MenuApiMap{
			MenuId: menuID,
			ApiId:  apiID,
		})
	}
	return mappings
}
