package menu

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
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

// Create 新增菜单。
func (s *MenuService) Create(params *form.CreateMenu) error {
	return s.edit(&menuMutation{
		Icon:            params.Icon,
		Title:           params.Title,
		Code:            params.Code,
		Path:            params.Path,
		Name:            params.Name,
		AnimateEnter:    params.AnimateEnter,
		AnimateLeave:    params.AnimateLeave,
		AnimateDuration: params.AnimateDuration,
		IsShow:          params.IsShow,
		IsAuth:          params.IsAuth,
		IsNewWindow:     params.IsNewWindow,
		Sort:            params.Sort,
		Type:            params.Type,
		Pid:             params.Pid,
		Description:     params.Description,
		ApiList:         params.ApiList,
		Component:       params.Component,
		Status:          params.Status,
		Redirect:        params.Redirect,
		IsExternalLinks: params.IsExternalLinks,
	})
}

// Update 更新菜单。
func (s *MenuService) Update(params *form.UpdateMenu) error {
	return s.edit(&menuMutation{
		Id:              params.Id,
		Icon:            params.Icon,
		Title:           params.Title,
		Code:            params.Code,
		Path:            params.Path,
		Name:            params.Name,
		AnimateEnter:    params.AnimateEnter,
		AnimateLeave:    params.AnimateLeave,
		AnimateDuration: params.AnimateDuration,
		IsShow:          params.IsShow,
		IsAuth:          params.IsAuth,
		IsNewWindow:     params.IsNewWindow,
		Sort:            params.Sort,
		Type:            params.Type,
		Pid:             params.Pid,
		Description:     params.Description,
		ApiList:         params.ApiList,
		Component:       params.Component,
		Status:          params.Status,
		Redirect:        params.Redirect,
		IsExternalLinks: params.IsExternalLinks,
	})
}

// Edit 兼容旧编辑入口，等同于更新。
func (s *MenuService) Edit(params *form.UpdateMenu) error {
	return s.Update(params)
}

type menuMutation struct {
	Id              uint
	Icon            string
	Title           string
	Code            string
	Path            string
	Name            string
	AnimateEnter    string
	AnimateLeave    string
	AnimateDuration float32
	IsShow          uint8
	IsAuth          uint8
	IsNewWindow     uint8
	Sort            uint
	Type            uint8
	Pid             uint
	Description     string
	ApiList         []uint
	Component       string
	Status          uint8
	Redirect        string
	IsExternalLinks uint8
}

func (s *MenuService) edit(params *menuMutation) error {
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
	return s.executeEditTransaction(menu, params.ApiList, editContext)
}

// menuEditContext 菜单编辑上下文信息
type menuEditContext struct {
	originPids     string
	originPid      uint
	originFullPath string
	excludeWhere   string
}

// prepareEditContext 准备编辑上下文信息
func (s *MenuService) prepareEditContext(menu *model.Menu, params *menuMutation) (*menuEditContext, error) {
	ctx := &menuEditContext{
		originPids:     menuRootPid,
		originPid:      0,
		originFullPath: "",
		excludeWhere:   "",
	}

	// 编辑模式：加载现有菜单数据
	if params.Id > 0 {
		if err := menu.GetById(params.Id); err != nil || menu.ID == 0 {
			return nil, e.NewBusinessError(1, "编辑的菜单不存在")
		}
		ctx.originPids = menu.Pids
		ctx.originPid = menu.Pid
		ctx.originFullPath = menu.FullPath
		ctx.excludeWhere = fmt.Sprintf(" AND id != %d", params.Id)
	}

	return ctx, nil
}

// handleParentChange 处理父级菜单变化
func (s *MenuService) handleParentChange(menu *model.Menu, params *menuMutation) error {
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
func (s *MenuService) updateMenuWithParent(menu *model.Menu, params *menuMutation) error {
	parentMenu := model.NewMenu()
	if err := parentMenu.GetById(params.Pid); err != nil || parentMenu.ID == 0 {
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
func (s *MenuService) setRootMenuFields(menu *model.Menu, params *menuMutation) {
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
func (s *MenuService) assignMenuFields(menu *model.Menu, params *menuMutation) {
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
func (s *MenuService) validateUniqueFields(menu *model.Menu, params *menuMutation, excludeWhere string) error {
	// 验证权限标识唯一性
	codeExists, err := menu.Exists("code = ?"+excludeWhere, params.Code)
	if err != nil {
		return err
	}
	if params.Code != "" && codeExists {
		return e.NewBusinessError(1, "权限标识已存在")
	}

	// 验证路由名称唯一性
	nameExists, err := menu.Exists("name = ?"+excludeWhere, params.Name)
	if err != nil {
		return err
	}
	if params.Name != "" && nameExists {
		return e.NewBusinessError(1, "路由名称已存在")
	}

	// 验证路由路径唯一性（按钮类型不需要验证）
	if params.Type != model.BUTTON && menu.Path != "" {
		pathExists, err := menu.Exists("full_path = ?"+excludeWhere, menu.FullPath)
		if err != nil {
			return err
		}
		if pathExists {
			return e.NewBusinessError(1, "路由已存在")
		}
	}

	return nil
}

// validateAndFilterApiList 验证并过滤 API 列表
func (s *MenuService) validateAndFilterApiList(params *menuMutation) error {
	if len(params.ApiList) == 0 {
		return nil
	}

	var apis []model.Api
	apiDB, err := model.GetDB(model.NewApi())
	if err != nil {
		return err
	}
	if err := apiDB.
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
func (s *MenuService) executeEditTransaction(menu *model.Menu, apiList []uint, editContext *menuEditContext) error {
	db, err := menu.GetDB()
	if err != nil {
		return err
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		menu.SetDB(tx)

		// 保存菜单
		if err := menu.Save(); err != nil {
			return err
		}

		// 更新子菜单的层级信息
		if err := s.updateChildrenLevels(menu, editContext.originPids, editContext.originFullPath, tx); err != nil {
			return err
		}

		// 更新父级的 children_num
		// 如果 pid 发生变化，需要更新旧父级和新父级
		// 如果是新增操作（originPid = 0），只需要更新新父级
		if editContext.originPid > 0 && editContext.originPid != menu.Pid {
			// 更新旧父级的 children_num（编辑操作且父级发生变化时）
			if err := model.UpdateChildrenNum(model.NewMenu(), editContext.originPid, tx); err != nil {
				return err
			}
		}
		// 更新新父级的 children_num（新增或父级变化时都需要更新）
		if menu.Pid > 0 && menu.Pid != editContext.originPid {
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

	if err != nil {
		return err
	}

	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByMenus([]uint{menu.ID})
}

// updateChildrenLevels 更新子菜单的层级和路径信息。
func (s *MenuService) updateChildrenLevels(menu *model.Menu, originPids string, originFullPath string, tx *gorm.DB) error {
	if menu.Pids == originPids && menu.FullPath == originFullPath {
		return nil
	}

	var descendants []*model.Menu
	if err := tx.Where("FIND_IN_SET(?,pids)", menu.ID).Order("level asc, id asc").Find(&descendants).Error; err != nil {
		return err
	}
	if len(descendants) == 0 {
		return nil
	}

	childrenByPID := make(map[uint][]*model.Menu, len(descendants))
	for _, child := range descendants {
		childrenByPID[child.Pid] = append(childrenByPID[child.Pid], child)
	}

	return s.rebuildMenuDescendants(tx, menu, childrenByPID)
}

func (s *MenuService) rebuildMenuDescendants(tx *gorm.DB, parent *model.Menu, childrenByPID map[uint][]*model.Menu) error {
	children := childrenByPID[parent.ID]
	for _, child := range children {
		s.applyDescendantMenuState(parent, child)

		if err := tx.Model(model.NewMenu()).
			Where("id = ?", child.ID).
			Updates(map[string]any{
				"pids":      child.Pids,
				"level":     child.Level,
				"full_path": child.FullPath,
			}).Error; err != nil {
			return err
		}

		if err := s.rebuildMenuDescendants(tx, child, childrenByPID); err != nil {
			return err
		}
	}
	return nil
}

func (s *MenuService) applyDescendantMenuState(parent *model.Menu, child *model.Menu) {
	child.Pids = s.buildPids(parent.Pids, parent.ID)
	child.Level = parent.Level + 1
	child.FullPath = s.buildFullPath(child.Path, parent.FullPath, child.Type)
	if child.Type == model.BUTTON {
		child.FullPath = ""
	}
}

// updateMenuPermissions 更新菜单权限关联
func (s *MenuService) updateMenuPermissions(menu *model.Menu, apiList []uint, tx ...*gorm.DB) error {
	menuApiMap := model.NewMenuApiMap()
	if len(tx) > 0 {
		menuApiMap.SetDB(tx[0])
	}

	// 获取现有关联
	existingMaps, err := model.ListE(menuApiMap, "menu_id = ?", []any{menu.ID}, model.ListOptionalParams{
		SelectFields: []string{"api_id"},
	})
	if err != nil {
		return err
	}

	existingIds := lo.Map(existingMaps, func(m *model.MenuApiMap, _ int) uint {
		return m.ApiId
	})

	apiList = lo.Uniq(apiList)
	toDelete, toAdd := lo.Difference(existingIds, apiList)

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := menuApiMap.DeleteWhere(
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
		if err := menuApiMap.CreateBatch(newMappings); err != nil {
			return err
		}
	}
	return nil
}

// UpdateAllMenuPermissions 批量更新所有菜单的权限到 Casbin
func (s *MenuService) UpdateAllMenuPermissions() error {
	return access.NewPermissionSyncCoordinator().SyncAll()
}

// ListPage 分页查询菜单列表
func (s *MenuService) ListPage(params *form.ListMenu) *resources.Collection {
	condition, args := s.buildListCondition(params, false)

	menu := model.NewMenu()
	total, collection, err := model.ListPageE(menu, params.Page, params.PerPage, condition, args)
	if err != nil {
		return resources.NewMenuTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return resources.NewMenuTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

// List 查询菜单树形列表
func (s *MenuService) List(params *form.ListMenu) any {
	condition, args := s.buildListCondition(params, true)

	menus, err := model.ListE(model.NewMenu(), condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	if err != nil {
		return resources.NewMenuTreeTransformer().BuildTreeByNode(nil, 0)
	}

	return resources.NewMenuTreeTransformer().BuildTreeByNode(menus, 0)
}

// buildListCondition 构建列表查询条件
func (s *MenuService) buildListCondition(params *form.ListMenu, includeStatus bool) (string, []any) {
	qb := query_builder.New()
	if params.Keyword != "" {
		qb.AddCondition("(title like ? OR path like ? OR code = ?)", "%"+params.Keyword+"%", "%"+params.Keyword+"%", params.Keyword)
	}

	qb.AddEq("is_auth", params.IsAuth)
	if includeStatus && params.Status != nil && *params.Status != allStatus {
		qb.AddEq("status", params.Status)
	}

	return qb.Build()
}

// Delete 删除菜单
func (s *MenuService) Delete(id uint) error {
	menu := model.NewMenu()
	if err := menu.GetById(id); err != nil || menu.ID == 0 {
		return e.NewBusinessError(1, "菜单不存在")
	}

	// 检查是否有子菜单（使用 children_num 字段判断，性能更好）
	if menu.ChildrenNum > 0 {
		return e.NewBusinessError(1, "该菜单有子菜单，无法删除")
	}

	// 使用事务确保数据一致性
	db, err := menu.GetDB()
	if err != nil {
		return e.NewBusinessError(1, "删除菜单失败")
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		menu.SetDB(tx)

		// 保存父级ID，用于后续更新 children_num
		parentId := menu.Pid

		// 删除菜单
		if _, deleteErr := menu.DeleteByID(id); deleteErr != nil {
			return deleteErr
		}

		// 更新父级的 children_num
		if parentId > 0 {
			if err := model.UpdateChildrenNum(model.NewMenu(), parentId, tx); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return e.NewBusinessError(1, "删除菜单失败")
	}

	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByMenus([]uint{id})
}

// Detail 获取菜单详情
func (s *MenuService) Detail(id uint) (any, error) {
	menu := model.NewMenu()
	if err := menu.GetAllById(id); err != nil || menu.ID == 0 {
		return nil, e.NewBusinessError(1, "菜单不存在")
	}
	return resources.NewMenuTransformer().ToStruct(menu), nil
}
