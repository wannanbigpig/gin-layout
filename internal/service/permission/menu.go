package permission

import (
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	casbinx "github.com/wannanbigpig/gin-layout/internal/pkg/utils/casbin"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

const (
	menuRootPid   = "0"
	menuRootLevel = 1
	maxMenuLevel  = 4 // 最多4层菜单
	allStatus     = 2
	rootPath      = "/"
)

// MenuService 菜单服务
type MenuService struct {
	service.Base
}

// NewMenuService 创建菜单服务实例
func NewMenuService() *MenuService {
	return &MenuService{}
}

// Edit 编辑菜单（新增或更新）
func (s *MenuService) Edit(params *form.EditMenu) error {
	menu := model.NewMenu()
	editContext, err := s.prepareEditContext(menu, params)
	if err != nil {
		return err
	}

	// 处理父级菜单变化
	if err := s.handleParentChange(menu, params); err != nil {
		return err
	}

	// 验证层级限制
	if menu.Level > maxMenuLevel {
		return e.NewBusinessError(1, "最多只能创建4层菜单")
	}

	// 赋值菜单字段
	s.assignMenuFields(menu, params)

	// 验证字段唯一性
	if err := s.validateUniqueFields(menu, params, editContext.excludeWhere); err != nil {
		return err
	}

	// 验证并过滤 API 列表
	if err := s.validateAndFilterApiList(params); err != nil {
		return err
	}

	// 执行数据库事务操作
	return s.executeEditTransaction(menu, params.ApiList, editContext.originPids, editContext.originPid)
}

// menuEditContext 菜单编辑上下文信息
type menuEditContext struct {
	originPids   string
	originPid    uint
	excludeWhere string
}

// prepareEditContext 准备编辑上下文信息
func (s *MenuService) prepareEditContext(menu *model.Menu, params *form.EditMenu) (*menuEditContext, error) {
	ctx := &menuEditContext{
		originPids:   menuRootPid,
		originPid:    0,
		excludeWhere: "",
	}

	// 编辑模式：加载现有菜单数据
	if params.Id > 0 {
		if err := menu.GetById(menu, params.Id); err != nil || menu.ID == 0 {
			return nil, e.NewBusinessError(1, "编辑的菜单不存在")
		}
		ctx.originPids = menu.Pids
		ctx.originPid = menu.Pid
		ctx.excludeWhere = fmt.Sprintf(" AND id != %d", params.Id)
	}

	return ctx, nil
}

// handleParentChange 处理父级菜单变化
func (s *MenuService) handleParentChange(menu *model.Menu, params *form.EditMenu) error {
	// 判断是否需要处理父级变化
	needHandleParent := (params.Pid > 0 && params.Pid != menu.Pid) ||
		(params.Path != menu.Path && params.Pid > 0)

	if needHandleParent {
		return s.updateMenuWithParent(menu, params)
	}

	if params.Pid == 0 {
		s.setRootMenuFields(menu, params)
	}

	menu.Pid = params.Pid
	return nil
}

// updateMenuWithParent 更新有父级的菜单信息
func (s *MenuService) updateMenuWithParent(menu *model.Menu, params *form.EditMenu) error {
	var parentMenu model.Menu
	if err := parentMenu.GetById(&parentMenu, params.Pid); err != nil || parentMenu.ID == 0 {
		return e.NewBusinessError(1, "上级菜单不存在")
	}

	// 验证上级菜单类型
	if parentMenu.Type == model.BUTTON {
		return e.NewBusinessError(1, "上级菜单不能是按钮类型")
	}

	// 防止循环引用
	if utils2.WouldCauseCycle(menu.ID, params.Pid, parentMenu.Pids) {
		return e.NewBusinessError(1, "上级菜单不能是当前菜单自身或其子菜单")
	}

	// 更新菜单层级和路径信息
	menu.Level = parentMenu.Level + 1
	menu.Pids = s.buildPids(parentMenu.Pids, parentMenu.ID)
	menu.FullPath = s.buildFullPath(params.Path, parentMenu.FullPath, params.Type)
	menu.Pid = params.Pid

	return nil
}

// setRootMenuFields 设置根菜单字段
func (s *MenuService) setRootMenuFields(menu *model.Menu, params *form.EditMenu) {
	menu.Level = menuRootLevel
	menu.Pids = menuRootPid
	menu.FullPath = s.buildFullPath(params.Path, rootPath, params.Type)
	menu.Pid = 0
}

// buildPids 构建父级ID序列
func (s *MenuService) buildPids(parentPids string, parentID uint) string {
	return strings.TrimPrefix(fmt.Sprintf("%s,%d", parentPids, parentID), ",")
}

// buildFullPath 构建完整路径（按钮类型返回空字符串）
func (s *MenuService) buildFullPath(path, parentPath string, menuType uint8) string {
	if menuType == model.BUTTON {
		return ""
	}
	return s.assembleFullPath(path, parentPath)
}

// assembleFullPath 组装完整路径
func (s *MenuService) assembleFullPath(path, parentPath string) string {
	if parentPath == "" {
		parentPath = rootPath
	}

	// 绝对路径或外部链接直接返回
	if strings.HasPrefix(path, rootPath) ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "http://") {
		return path
	}

	// 确保父路径以 / 结尾
	if !strings.HasSuffix(parentPath, "/") {
		parentPath += "/"
	}

	return parentPath + path
}

// assignMenuFields 将表单字段赋值给菜单模型
func (s *MenuService) assignMenuFields(menu *model.Menu, params *form.EditMenu) {
	menu.Icon = params.Icon
	menu.Pid = params.Pid
	menu.Title = params.Title
	menu.Code = params.Code
	menu.Path = params.Path
	menu.Name = params.Name
	menu.Component = params.Component
	menu.Status = params.Status
	menu.Redirect = params.Redirect
	menu.AnimateEnter = params.AnimateEnter
	menu.AnimateLeave = params.AnimateLeave
	menu.AnimateDuration = params.AnimateDuration
	menu.IsShow = params.IsShow
	menu.IsAuth = params.IsAuth
	menu.IsNewWindow = params.IsNewWindow
	menu.Sort = params.Sort
	menu.Type = params.Type
	menu.Description = params.Description
	menu.IsExternalLinks = params.IsExternalLinks

	// 按钮类型的 full_path 必须为空
	if params.Type == model.BUTTON {
		menu.FullPath = ""
	}
}

// validateUniqueFields 验证字段唯一性
func (s *MenuService) validateUniqueFields(menu *model.Menu, params *form.EditMenu, excludeWhere string) error {
	// 验证权限标识唯一性
	if params.Code != "" && menu.Exists(menu, "code = ?"+excludeWhere, params.Code) {
		return e.NewBusinessError(1, "权限标识已存在")
	}

	// 验证路由名称唯一性
	if params.Name != "" && menu.Exists(menu, "name = ?"+excludeWhere, params.Name) {
		return e.NewBusinessError(1, "路由名称已存在")
	}

	// 验证路由路径唯一性（按钮类型不需要验证）
	if params.Type != model.BUTTON && menu.Path != "" {
		if menu.Exists(menu, "full_path = ?"+excludeWhere, menu.FullPath) {
			return e.NewBusinessError(1, "路由已存在")
		}
	}

	return nil
}

// validateAndFilterApiList 验证并过滤 API 列表
func (s *MenuService) validateAndFilterApiList(params *form.EditMenu) error {
	if len(params.ApiList) == 0 {
		return nil
	}

	var apis []model.Api
	if err := model.NewApi().DB().
		Where("id IN ?", params.ApiList).
		Find(&apis).Error; err != nil {
		return err
	}

	// 提取有效的 API ID
	params.ApiList = lo.Map(apis, func(api model.Api, _ int) uint {
		return api.ID
	})

	return nil
}

// executeEditTransaction 执行编辑事务
func (s *MenuService) executeEditTransaction(menu *model.Menu, apiList []uint, originPids string, originPid uint) error {
	err := menu.DB().Transaction(func(tx *gorm.DB) error {
		// 保存菜单
		if err := tx.Save(menu).Error; err != nil {
			return err
		}

		// 更新子菜单的层级信息
		if err := s.updateChildrenLevels(menu, originPids, tx); err != nil {
			return err
		}

		// 更新父级的 children_num
		// 如果 pid 发生变化，需要更新旧父级和新父级
		// 如果是新增操作（originPid = 0），只需要更新新父级
		if originPid > 0 && originPid != menu.Pid {
			// 更新旧父级的 children_num（编辑操作且父级发生变化时）
			if err := model.UpdateChildrenNum(model.NewMenu(), originPid, tx); err != nil {
				return err
			}
		}
		// 更新新父级的 children_num（新增或父级变化时都需要更新）
		if menu.Pid > 0 && menu.Pid != originPid {
			if err := model.UpdateChildrenNum(model.NewMenu(), menu.Pid, tx); err != nil {
				return err
			}
		}

		// 更新菜单权限关联
		if err := s.updateMenuPermissions(menu, apiList, tx); err != nil {
			return err
		}

		return nil
	})

	// 事务失败时重新加载策略
	if err != nil {
		_ = casbinx.GetEnforcer().LoadPolicy()
	}

	return err
}

// updateChildrenLevels 更新子菜单的层级和路径信息
func (s *MenuService) updateChildrenLevels(menu *model.Menu, originPids string, tx *gorm.DB) error {
	if menu.Pids == originPids {
		return nil
	}

	return tx.Model(model.NewMenu()).
		Where("FIND_IN_SET(?,pids)", menu.ID).
		Updates(map[string]interface{}{
			"pids":  gorm.Expr("replace(pids,?,?)", originPids, menu.Pids),
			"level": gorm.Expr("length(pids) - length(replace(pids, ',', '')) + 1"),
		}).Error
}

// updateMenuPermissions 更新菜单权限关联
func (s *MenuService) updateMenuPermissions(menu *model.Menu, apiList []uint, tx ...*gorm.DB) error {
	menuApiMap := model.NewMenuApiMap()
	if len(tx) > 0 {
		menuApiMap.SetDB(tx[0])
	}

	// 获取现有关联
	existingMaps := model.List(menuApiMap, "menu_id = ?", []any{menu.ID}, model.ListOptionalParams{
		SelectFields: []string{"api_id"},
	})

	existingIds := lo.Map(existingMaps, func(m *model.MenuApiMap, _ int) uint {
		return m.ApiId
	})

	apiList = lo.Uniq(apiList)
	toDelete, toAdd := lo.Difference(existingIds, apiList)

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := menuApiMap.DeleteWithCondition(
			menuApiMap,
			"menu_id = ? AND api_id IN (?)",
			[]any{menu.ID, toDelete}...,
		); err != nil {
			return err
		}
	}

	// 批量创建新关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(apiId uint, _ int) *model.MenuApiMap {
			return &model.MenuApiMap{
				MenuId: menu.ID,
				ApiId:  apiId,
			}
		})
		if err := menuApiMap.BatchCreate(newMappings); err != nil {
			return err
		}
	}
	// 更新 Casbin 策略
	return s.updateCasbinPolicy(menu, apiList, tx...)
}

// updateCasbinPolicy 更新 Casbin 权限策略
func (s *MenuService) updateCasbinPolicy(menu *model.Menu, apiList []uint, tx ...*gorm.DB) error {
	// 查询 apiList 中的所有 API 信息（包括新增的和已存在的）
	if len(apiList) == 0 {
		// 如果没有 API，清空策略
		return s.editMenuPolicyPermissions(menu.ID, [][]string{}, tx...)
	}

	// 批量查询所有 API 信息
	apis := model.List(model.NewApi(), "id IN ?", []any{apiList}, model.ListOptionalParams{
		SelectFields: []string{"id", "route", "method"},
	})

	// 构建策略规则
	policy := lo.Map(apis, func(api *model.Api, _ int) []string {
		return []string{api.Route, api.Method}
	})

	return s.editMenuPolicyPermissions(menu.ID, policy, tx...)
}

// editMenuPolicyPermissions 编辑菜单的权限策略
func (s *MenuService) editMenuPolicyPermissions(menuId uint, policy [][]string, tx ...*gorm.DB) error {
	enforcer := casbinx.GetEnforcer()
	if enforcer.Error() != nil {
		return e.NewBusinessError(1, "编辑失败")
	}
	// 设置事务，如果没有传入事务则清理之前的事务状态
	if len(tx) > 0 {
		enforcer.SetDB(tx[0])
	} else {
		enforcer.SetDB(nil)
	}
	menuName := fmt.Sprintf("%s%s%d", global.CasbinMenuPrefix, global.CasbinSeparator, menuId)

	err := enforcer.EditPolicyPermissions(menuName, policy)
	if err != nil {
		return e.NewBusinessError(1, "编辑失败~")
	}
	return nil
}

// UpdateAllMenuPermissions 批量更新所有菜单的权限到 Casbin
func (s *MenuService) UpdateAllMenuPermissions() error {
	// 获取所有菜单与 API 的映射关系
	allMappings := model.List(model.NewMenuApiMap(), "", nil)

	// 提取并去重所有 API ID
	apiIDs := lo.Uniq(lo.Map(allMappings, func(m *model.MenuApiMap, _ int) uint {
		return m.ApiId
	}))

	// 批量查询所有 API
	apiList := model.List(model.NewApi(), "id IN ?", []any{apiIDs}, model.ListOptionalParams{
		SelectFields: []string{"id", "route", "method"},
	})

	// 构建 API 映射表
	apiMap := lo.SliceToMap(apiList, func(a *model.Api) (uint, *model.Api) {
		return a.ID, a
	})

	// 构建菜单权限策略映射
	menuPolicyMap := s.buildMenuPolicyMap(allMappings, apiMap)

	// 批量更新到 Casbin
	return s.batchUpdateCasbinPolicies(menuPolicyMap)
}

// buildMenuPolicyMap 构建菜单权限策略映射
func (s *MenuService) buildMenuPolicyMap(
	allMappings []*model.MenuApiMap,
	apiMap map[uint]*model.Api,
) map[uint][][]string {
	menuPolicyMap := make(map[uint][][]string)
	for _, m := range allMappings {
		if api, ok := apiMap[m.ApiId]; ok {
			menuPolicyMap[m.MenuId] = append(
				menuPolicyMap[m.MenuId],
				[]string{api.Route, api.Method},
			)
		}
	}
	return menuPolicyMap
}

// batchUpdateCasbinPolicies 批量更新 Casbin 策略
func (s *MenuService) batchUpdateCasbinPolicies(menuPolicyMap map[uint][][]string) error {
	err := model.DB().Transaction(func(tx *gorm.DB) error {
		for menuID, policy := range menuPolicyMap {
			if err := s.editMenuPolicyPermissions(menuID, policy, tx); err != nil {
				return e.NewBusinessError(1, "更新菜单权限失败")
			}
		}
		return nil
	})

	if err != nil {
		_ = casbinx.GetEnforcer().LoadPolicy()
		return e.NewBusinessError(1, "更新菜单权限失败")
	}

	return nil
}

// ListPage 分页查询菜单列表
func (s *MenuService) ListPage(params *form.ListMenu) *resources.Collection {
	condition, args := s.buildListCondition(params, false)

	menu := model.NewMenu()
	total, collection := model.ListPage(menu, params.Page, params.PerPage, condition, args)
	return resources.NewMenuTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

// List 查询菜单树形列表
func (s *MenuService) List(params *form.ListMenu) any {
	condition, args := s.buildListCondition(params, true)

	menus := model.List(model.NewMenu(), condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})

	return resources.NewMenuTreeTransformer().BuildTreeByNode(menus, 0)
}

// buildListCondition 构建列表查询条件
func (s *MenuService) buildListCondition(params *form.ListMenu, includeStatus bool) (string, []any) {
	var condition strings.Builder
	var args []any

	// 关键词搜索
	if params.Keyword != "" {
		condition.WriteString("(title like ? OR path like ? OR code = ?) AND ")
		args = append(args, "%"+params.Keyword+"%", "%"+params.Keyword+"%", params.Keyword)
	}

	// 鉴权状态过滤
	if params.IsAuth != nil {
		condition.WriteString("is_auth = ? AND ")
		args = append(args, params.IsAuth)
	}

	// 状态过滤（仅列表查询）
	if includeStatus && params.Status != nil && *params.Status != allStatus {
		condition.WriteString("status = ? AND ")
		args = append(args, params.Status)
	}

	return condition.String(), args
}

// Delete 删除菜单
func (s *MenuService) Delete(id uint) error {
	menu := model.NewMenu()
	if err := menu.GetById(menu, id); err != nil || menu.ID == 0 {
		return e.NewBusinessError(1, "菜单不存在")
	}

	// 检查是否有子菜单（使用 children_num 字段判断，性能更好）
	if menu.ChildrenNum > 0 {
		return e.NewBusinessError(1, "该菜单有子菜单，无法删除")
	}

	// 使用事务确保数据一致性
	err := menu.DB().Transaction(func(tx *gorm.DB) error {
		menu.SetDB(tx)

		// 保存父级ID，用于后续更新 children_num
		parentId := menu.Pid

		// 删除菜单
		if _, deleteErr := menu.Delete(menu, id); deleteErr != nil {
			return deleteErr
		}

		// 更新父级的 children_num
		if parentId > 0 {
			if err := model.UpdateChildrenNum(model.NewMenu(), parentId, tx); err != nil {
				return err
			}
		}

		// 删除菜单的Casbin策略
		return s.deleteAllPoliciesForMenu(id, tx)
	})

	if err != nil {
		// 如果事务失败，重新加载策略以确保一致性
		_ = casbinx.GetEnforcer().LoadPolicy()
		return e.NewBusinessError(1, "删除菜单失败")
	}

	return nil
}

// deleteAllPoliciesForMenu 删除菜单的所有Casbin策略
// 包括：
// 1. 菜单的权限策略（p策略：[p, menu:id, route, method]）
// 2. 所有角色引用该菜单的策略（g策略：[g, role:*, menu:id]）
func (s *MenuService) deleteAllPoliciesForMenu(menuId uint, tx *gorm.DB) error {
	enforcer := casbinx.GetEnforcer()
	if enforcer == nil || enforcer.Error() != nil {
		return nil
	}

	enforcer.SetDB(tx)
	menuName := fmt.Sprintf("%s%s%d", global.CasbinMenuPrefix, global.CasbinSeparator, menuId)

	return enforcer.WithTransaction(func(e casbin.IEnforcer) error {
		// 删除菜单的权限策略（p策略：[p, menu:id, route, method]）
		_, _ = e.DeletePermissionsForUser(menuName)

		// 删除所有角色引用该菜单的策略（g策略：[g, role:*, menu:id]）
		// 使用 RemoveFilteredGroupingPolicy 删除所有第二个参数匹配的策略
		_, _ = e.RemoveFilteredGroupingPolicy(1, menuName)

		return nil
	})
}

// Detail 获取菜单详情
func (s *MenuService) Detail(id uint) (any, error) {
	menu := model.NewMenu()
	if err := menu.GetAllById(menu, id); err != nil || menu.ID == 0 {
		return nil, e.NewBusinessError(1, "菜单不存在")
	}
	return resources.NewMenuTransformer().ToStruct(menu), nil
}
