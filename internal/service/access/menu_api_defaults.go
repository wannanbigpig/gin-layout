package access

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/samber/lo"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

type defaultMenuAPIBinding struct {
	MenuCode string
	Route    string
	Method   string
}

var defaultMenuAPIBindings = []defaultMenuAPIBinding{
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
}

// MenuAPIDefaultsService 负责初始化默认菜单与接口映射关系。
type MenuAPIDefaultsService struct{}

// NewMenuAPIDefaultsService 创建默认菜单接口映射服务实例。
func NewMenuAPIDefaultsService() *MenuAPIDefaultsService {
	return &MenuAPIDefaultsService{}
}

// Sync 将默认菜单接口映射写入数据库。
func (s *MenuAPIDefaultsService) Sync(tx ...*gorm.DB) error {
	execTx := FirstTx(tx)
	if execTx == nil {
		db, err := model.GetDB()
		if err != nil {
			return err
		}
		execTx = db
	}

	menuCodes := lo.Uniq(lo.Map(defaultMenuAPIBindings, func(item defaultMenuAPIBinding, _ int) string {
		return item.MenuCode
	}))
	routes := lo.Uniq(lo.Map(defaultMenuAPIBindings, func(item defaultMenuAPIBinding, _ int) string {
		return item.Route
	}))
	methods := lo.Uniq(lo.Map(defaultMenuAPIBindings, func(item defaultMenuAPIBinding, _ int) string {
		return item.Method
	}))

	var menus []*model.Menu
	if err := execTx.Select("id", "code").
		Where("code IN ? AND deleted_at = 0", menuCodes).
		Find(&menus).Error; err != nil {
		return err
	}
	menuIDByCode := make(map[string]uint, len(menus))
	for _, menu := range menus {
		menuIDByCode[menu.Code] = menu.ID
	}

	var apis []*model.Api
	if err := execTx.Select("id", "route", "method").
		Where("route IN ? AND method IN ? AND deleted_at = 0", routes, methods).
		Find(&apis).Error; err != nil {
		return err
	}
	apiIDByRouteMethod := make(map[string]uint, len(apis))
	for _, api := range apis {
		apiIDByRouteMethod[api.Method+":"+api.Route] = api.ID
	}

	mappings := make([]*model.MenuApiMap, 0, len(defaultMenuAPIBindings))
	for _, item := range defaultMenuAPIBindings {
		menuID, ok := menuIDByCode[item.MenuCode]
		if !ok {
			continue
		}
		apiID, ok := apiIDByRouteMethod[item.Method+":"+item.Route]
		if !ok {
			continue
		}
		mappings = append(mappings, &model.MenuApiMap{
			MenuId: menuID,
			ApiId:  apiID,
		})
	}
	if len(mappings) == 0 {
		return nil
	}

	return execTx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "menu_id"}, {Name: "api_id"}},
		DoNothing: true,
	}).Create(&mappings).Error
}
