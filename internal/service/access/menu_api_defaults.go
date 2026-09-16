package access

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

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
	{MenuCode: "adminUser:bindRole", Route: "/admin/v1/role/options", Method: "GET"},
	{MenuCode: "adminUser:delete", Route: "/admin/v1/admin-user/delete", Method: "POST"},
	{MenuCode: "adminUser:list", Route: "/admin/v1/admin-user/get-full-phone", Method: "GET"},
	{MenuCode: "adminUser:list", Route: "/admin/v1/admin-user/get-full-email", Method: "GET"},
	{MenuCode: "menu:add", Route: "/admin/v1/menu/create", Method: "POST"},
	{MenuCode: "menu:add", Route: "/admin/v1/permission/list", Method: "GET"},
	{MenuCode: "menu:add", Route: "/admin/v1/permission/options", Method: "GET"},
	{MenuCode: "menu:addChild", Route: "/admin/v1/menu/create", Method: "POST"},
	{MenuCode: "menu:addChild", Route: "/admin/v1/permission/list", Method: "GET"},
	{MenuCode: "menu:addChild", Route: "/admin/v1/permission/options", Method: "GET"},
	{MenuCode: "menu:update", Route: "/admin/v1/menu/detail", Method: "GET"},
	{MenuCode: "menu:update", Route: "/admin/v1/menu/update", Method: "POST"},
	{MenuCode: "menu:update", Route: "/admin/v1/permission/list", Method: "GET"},
	{MenuCode: "menu:update", Route: "/admin/v1/permission/options", Method: "GET"},
	{MenuCode: "menu:refreshPermissionCache", Route: "/admin/v1/menu/update-all-menu-permissions", Method: "POST"},
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
	{MenuCode: "department:bindRole", Route: "/admin/v1/role/options", Method: "GET"},
	{MenuCode: "department:delete", Route: "/admin/v1/department/delete", Method: "POST"},
	{MenuCode: "api:update", Route: "/admin/v1/permission/update", Method: "POST"},
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
	{MenuCode: "requestLog:list", Route: "/admin/v1/admin-user/options", Method: "GET"},
	{MenuCode: "requestLog:export", Route: "/admin/v1/log/request/export", Method: "POST"},
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
	{MenuCode: "file:list", Route: "/admin/v1/system/file/list", Method: "GET"},
	{MenuCode: "file:list", Route: "/admin/v1/system/file/detail", Method: "GET"},
	{MenuCode: "file:list", Route: "/admin/v1/system/file/folder/tree", Method: "GET"},
	{MenuCode: "file:list", Route: "/admin/v1/system/file/folder/stats", Method: "GET"},
	{MenuCode: "file:list", Route: "/admin/v1/system/file/trash/list", Method: "GET"},
	{MenuCode: "file:list", Route: "/admin/v1/system/file/references", Method: "GET"},
	{MenuCode: "file:list", Route: "/admin/v1/system/file/export", Method: "POST"},
	{MenuCode: "file:list", Route: "/admin/v1/system/export/list", Method: "GET"},
	{MenuCode: "file:list", Route: "/admin/v1/system/export/detail", Method: "GET"},
	{MenuCode: "file:create", Route: "/admin/v1/system/file/upload/options", Method: "GET"},
	{MenuCode: "file:create", Route: "/admin/v1/system/file/upload/credential", Method: "POST"},
	{MenuCode: "file:create", Route: "/admin/v1/system/file/upload/credential/batch", Method: "POST"},
	{MenuCode: "file:create", Route: "/admin/v1/system/file/upload/complete", Method: "POST"},
	{MenuCode: "file:create", Route: "/admin/v1/system/file/upload/complete/batch", Method: "POST"},
	{MenuCode: "file:create", Route: "/admin/v1/system/file/upload/local", Method: "POST"},
	{MenuCode: "file:create", Route: "/admin/v1/system/file/upload/multipart/init", Method: "POST"},
	{MenuCode: "file:create", Route: "/admin/v1/system/file/upload/multipart/complete", Method: "POST"},
	{MenuCode: "file:create", Route: "/admin/v1/system/file/upload/multipart/abort", Method: "POST"},
	{MenuCode: "file:update", Route: "/admin/v1/system/file/folder/create", Method: "POST"},
	{MenuCode: "file:update", Route: "/admin/v1/system/file/folder/update", Method: "POST"},
	{MenuCode: "file:delete", Route: "/admin/v1/system/file/folder/delete", Method: "POST"},
	{MenuCode: "file:update", Route: "/admin/v1/system/file/folder/move", Method: "POST"},
	{MenuCode: "file:update", Route: "/admin/v1/system/file/update", Method: "POST"},
	{MenuCode: "file:update", Route: "/admin/v1/system/file/move", Method: "POST"},
	{MenuCode: "file:delete", Route: "/admin/v1/system/file/delete", Method: "POST"},
	{MenuCode: "file:delete", Route: "/admin/v1/system/file/batch-delete", Method: "POST"},
	{MenuCode: "file:restore", Route: "/admin/v1/system/file/trash/restore", Method: "POST"},
	{MenuCode: "file:restore", Route: "/admin/v1/system/file/trash/batch-restore", Method: "POST"},
	{MenuCode: "file:destroy", Route: "/admin/v1/system/file/trash/destroy", Method: "POST"},
	{MenuCode: "file:destroy", Route: "/admin/v1/system/file/trash/batch-destroy", Method: "POST"},
	{MenuCode: "storage:config", Route: "/admin/v1/system/storage/config", Method: "GET"},
	{MenuCode: "storage:secret", Route: "/admin/v1/system/storage/secret", Method: "GET"},
	{MenuCode: "storage:update", Route: "/admin/v1/system/storage/config", Method: "POST"},
	{MenuCode: "storage:test", Route: "/admin/v1/system/storage/test", Method: "POST"},
	{MenuCode: "session:list", Route: "/admin/v1/auth/session/list", Method: "GET"},
	{MenuCode: "session:revoke", Route: "/admin/v1/auth/session/revoke", Method: "POST"},
	{MenuCode: "task:list", Route: "/admin/v1/task/list", Method: "GET"},
	{MenuCode: "task:update", Route: "/admin/v1/task/record-policy", Method: "POST"},
	{MenuCode: "task:update", Route: "/admin/v1/task/operation-config", Method: "POST"},
	{MenuCode: "task:list", Route: "/admin/v1/task/run/list", Method: "GET"},
	{MenuCode: "task:stats", Route: "/admin/v1/task/run/stats", Method: "GET"},
	{MenuCode: "task:stats", Route: "/admin/v1/task/run/stats/trend", Method: "GET"},
	{MenuCode: "task:detail", Route: "/admin/v1/task/run/detail", Method: "GET"},
	{MenuCode: "task:detail", Route: "/admin/v1/task/run/events", Method: "GET"},
	{MenuCode: "task:list", Route: "/admin/v1/task/cron/state", Method: "GET"},
	{MenuCode: "task:list", Route: "/admin/v1/system/export/list", Method: "GET"},
	{MenuCode: "task:list", Route: "/admin/v1/system/export/detail", Method: "GET"},
	{MenuCode: "task:trigger", Route: "/admin/v1/task/trigger", Method: "POST"},
	{MenuCode: "task:retry", Route: "/admin/v1/task/run/retry", Method: "POST"},
	{MenuCode: "task:cancel", Route: "/admin/v1/task/run/cancel", Method: "POST"},
	{MenuCode: "requestLog:export", Route: "/admin/v1/system/export/list", Method: "GET"},
	{MenuCode: "requestLog:export", Route: "/admin/v1/system/export/detail", Method: "GET"},
	{MenuCode: "notification:send", Route: "/admin/v1/system/notification/send", Method: "POST"},
	{MenuCode: "dashboard:overview", Route: "/admin/v1/dashboard/overview", Method: "GET"},
	{MenuCode: "dashboard:statistics", Route: "/admin/v1/dashboard/statistics", Method: "GET"},
	{MenuCode: "dashboard:monitor", Route: "/admin/v1/dashboard/monitor", Method: "GET"},
	{MenuCode: "product:list", Route: "/admin/v1/product/list", Method: "GET"},
	{MenuCode: "product:list", Route: "/admin/v1/product/detail", Method: "GET"},
	{MenuCode: "product:create", Route: "/admin/v1/product/create", Method: "POST"},
	{MenuCode: "product:update", Route: "/admin/v1/product/detail", Method: "GET"},
	{MenuCode: "product:update", Route: "/admin/v1/product/update", Method: "POST"},
	{MenuCode: "product:delete", Route: "/admin/v1/product/delete", Method: "POST"},
}

// MenuAPIDefaultsService 负责初始化默认菜单与接口映射关系。
type MenuAPIDefaultsService struct {
	// bindings 默认菜单与接口绑定配置（合并后）。
	bindings []defaultMenuAPIBinding
	// builtinBindings 内置默认绑定。
	builtinBindings []defaultMenuAPIBinding
	// externalBindings 外部配置绑定。
	externalBindings []ExternalMenuAPIBinding
	// includeExternal 是否包含外部绑定。
	includeExternal bool
}

// MenuAPIDefaultsServiceDeps 描述 MenuAPIDefaultsService 可注入依赖。
type MenuAPIDefaultsServiceDeps struct {
	// Bindings 自定义默认菜单接口绑定（覆盖内置）。
	Bindings []defaultMenuAPIBinding
	// ExternalBindings 外部配置绑定。
	ExternalBindings []ExternalMenuAPIBinding
	// IncludeExternal 是否包含外部绑定，默认 true。
	IncludeExternal *bool
}

// NewMenuAPIDefaultsService 创建默认菜单接口映射服务实例。
func NewMenuAPIDefaultsService() *MenuAPIDefaultsService {
	return NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{})
}

// NewMenuAPIDefaultsServiceWithDeps 创建带依赖注入的默认菜单接口映射服务实例。
func NewMenuAPIDefaultsServiceWithDeps(deps MenuAPIDefaultsServiceDeps) *MenuAPIDefaultsService {
	s := &MenuAPIDefaultsService{
		includeExternal: true,
	}

	if deps.IncludeExternal != nil {
		s.includeExternal = *deps.IncludeExternal
	}

	if deps.Bindings != nil {
		s.builtinBindings = cloneMenuAPIBindings(deps.Bindings)
	} else {
		s.builtinBindings = defaultMenuAPIBindings()
	}

	if deps.ExternalBindings != nil {
		s.externalBindings = cloneExternalMenuAPIBindings(deps.ExternalBindings)
	} else if s.includeExternal {
		s.externalBindings = LoadExternalBindings()
	}

	s.bindings = MergeBindings(s.builtinBindings, s.externalBindings)
	return s
}

// cloneExternalMenuAPIBindings 克隆外部绑定切片。
func cloneExternalMenuAPIBindings(source []ExternalMenuAPIBinding) []ExternalMenuAPIBinding {
	if len(source) == 0 {
		return nil
	}
	cloned := make([]ExternalMenuAPIBinding, len(source))
	copy(cloned, source)
	return cloned
}

// BuiltinBindings 返回内置绑定列表。
func (s *MenuAPIDefaultsService) BuiltinBindings() []defaultMenuAPIBinding {
	return cloneMenuAPIBindings(s.builtinBindings)
}

// ExternalBindings 返回外部绑定列表。
func (s *MenuAPIDefaultsService) ExternalBindings() []ExternalMenuAPIBinding {
	return cloneExternalMenuAPIBindings(s.externalBindings)
}

// Bindings 返回合并后的绑定列表。
func (s *MenuAPIDefaultsService) Bindings() []defaultMenuAPIBinding {
	return cloneMenuAPIBindings(s.bindings)
}

// BindingStats 返回绑定统计信息。
func (s *MenuAPIDefaultsService) BindingStats() map[string]int {
	return GetBindingStats(s.builtinBindings, s.externalBindings)
}

// AddExternalBinding 添加单个外部绑定并重新合并。
func (s *MenuAPIDefaultsService) AddExternalBinding(binding ExternalMenuAPIBinding) {
	if binding.Source == "" {
		binding.Source = BindingSourceExternal
	}
	s.externalBindings = append(s.externalBindings, binding)
	s.bindings = MergeBindings(s.builtinBindings, s.externalBindings)
}

// AddExternalBindings 批量添加外部绑定并重新合并。
func (s *MenuAPIDefaultsService) AddExternalBindings(bindings []ExternalMenuAPIBinding) {
	for i := range bindings {
		if bindings[i].Source == "" {
			bindings[i].Source = BindingSourceExternal
		}
	}
	s.externalBindings = append(s.externalBindings, bindings...)
	s.bindings = MergeBindings(s.builtinBindings, s.externalBindings)
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

// Sync 将默认菜单接口映射同步到数据库，补齐新增默认关系并删除过期默认关系。
func (s *MenuAPIDefaultsService) Sync(tx ...*gorm.DB) error {
	db := FirstTx(tx)
	if db == nil {
		var err error
		db, err = model.NewMenuApiMap().GetDB()
		if err != nil {
			return err
		}
	}

	if FirstTx(tx) != nil {
		return s.syncWithDB(db)
	}
	return db.Transaction(func(execTx *gorm.DB) error {
		return s.syncWithDB(execTx)
	})
}

func (s *MenuAPIDefaultsService) syncWithDB(db *gorm.DB) error {
	bindings := s.bindings
	var mappings []*model.MenuApiMap
	if len(bindings) > 0 {
		menuCodes, routes, methods := collectMenuAPIBindingKeys(bindings)
		targets, err := loadDefaultMenuAPITargets(db, menuCodes, routes, methods)
		if err != nil {
			return fmt.Errorf("load default menu/api targets: %w", err)
		}
		mappings = buildDefaultMenuAPIMappings(bindings, targets)
	}
	if err := reconcileDefaultMenuAPIMappings(db, mappings); err != nil {
		return fmt.Errorf("reconcile default menu/api mappings: %w", err)
	}
	return nil
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

type defaultMenuAPIMappingKey struct {
	menuID uint
	apiID  uint
}

type defaultMenuAPIRow struct {
	ID     uint
	MenuID uint `gorm:"column:menu_id"`
	APIID  uint `gorm:"column:api_id"`
}

func (defaultMenuAPIRow) TableName() string {
	return "menu_api_map"
}

type defaultMenuAPIWriteRow struct {
	ID        uint
	MenuID    uint      `gorm:"column:menu_id"`
	APIID     uint      `gorm:"column:api_id"`
	IsDefault uint8     `gorm:"column:is_default"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (defaultMenuAPIWriteRow) TableName() string {
	return "menu_api_map"
}

func reconcileDefaultMenuAPIMappings(db *gorm.DB, mappings []*model.MenuApiMap) error {
	desiredSet := make(map[defaultMenuAPIMappingKey]struct{}, len(mappings))
	now := time.Now()
	for _, mapping := range mappings {
		if mapping == nil || mapping.MenuId == 0 || mapping.ApiId == 0 {
			continue
		}
		key := defaultMenuAPIMappingKey{menuID: mapping.MenuId, apiID: mapping.ApiId}
		if _, ok := desiredSet[key]; ok {
			continue
		}
		desiredSet[key] = struct{}{}
		if err := upsertDefaultMenuAPIMapping(db, mapping.MenuId, mapping.ApiId, now); err != nil {
			return fmt.Errorf("upsert default mapping menu_id=%d api_id=%d: %w", mapping.MenuId, mapping.ApiId, err)
		}
	}

	var existing []defaultMenuAPIRow
	if err := db.Table("menu_api_map").
		Select("id", "menu_id", "api_id").
		Where("is_default = ?", 1).
		Find(&existing).Error; err != nil {
		return fmt.Errorf("load existing default mappings: %w", err)
	}

	staleIDs := make([]uint, 0)
	for _, row := range existing {
		key := defaultMenuAPIMappingKey{menuID: row.MenuID, apiID: row.APIID}
		if _, ok := desiredSet[key]; !ok {
			staleIDs = append(staleIDs, row.ID)
		}
	}
	if len(staleIDs) == 0 {
		return nil
	}
	if err := db.Where("id IN ?", staleIDs).Delete(&defaultMenuAPIRow{}).Error; err != nil {
		return fmt.Errorf("delete stale default mappings: %w", err)
	}
	return nil
}

func upsertDefaultMenuAPIMapping(db *gorm.DB, menuID, apiID uint, now time.Time) error {
	var existing defaultMenuAPIRow
	err := db.Where("menu_id = ? AND api_id = ?", menuID, apiID).First(&existing).Error
	if err == nil {
		return db.Model(&defaultMenuAPIWriteRow{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{"is_default": 1, "updated_at": now}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&defaultMenuAPIWriteRow{
		MenuID:    menuID,
		APIID:     apiID,
		IsDefault: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error
}

func defaultMenuAPIBindingRouteSet(bindings []defaultMenuAPIBinding) map[string]struct{} {
	result := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		result[binding.Method+" "+binding.Route] = struct{}{}
	}
	return result
}

// DefaultMenuAPIBindingRouteSet 返回所有默认菜单/API 绑定（含外部）的 method+route 集合。
func DefaultMenuAPIBindingRouteSet() map[string]struct{} {
	external := LoadExternalBindings()
	if len(external) == 0 {
		return defaultMenuAPIBindingRouteSet(defaultMenuAPIBindings())
	}
	merged := MergeBindings(defaultMenuAPIBindings(), external)
	return defaultMenuAPIBindingRouteSet(merged)
}

// BuiltinDefaultMenuAPIBindingRouteSet 返回仅内置默认菜单/API 绑定的 method+route 集合。
func BuiltinDefaultMenuAPIBindingRouteSet() map[string]struct{} {
	return defaultMenuAPIBindingRouteSet(defaultMenuAPIBindings())
}
