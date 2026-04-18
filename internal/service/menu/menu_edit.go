package menu

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// Create 新增菜单。
func (s *MenuService) Create(params *form.CreateMenu) error {
	return s.applyMenuMutation(&menuMutation{
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
	return s.applyMenuMutation(&menuMutation{
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

// menuMutation 菜单变更参数，用于封装新增/更新菜单的请求数据。
type menuMutation struct {
	Id              uint     // 菜单 ID，0 表示新增
	Icon            string   // 图标
	Title           string   // 标题
	Code            string   // 权限标识
	Path            string   // 路径
	Name            string   // 路由名称
	AnimateEnter    string   // 进入动画
	AnimateLeave    string   // 离开动画
	AnimateDuration float32  // 动画时长
	IsShow          uint8    // 是否显示
	IsAuth          uint8    // 是否鉴权
	IsNewWindow     uint8    // 是否新窗口打开
	Sort            uint     // 排序权重
	Type            uint8    // 菜单类型（目录/菜单/按钮）
	Pid             uint     // 父菜单 ID
	Description     string   // 描述
	ApiList         []uint   // 关联的 API ID 列表
	Component       string   // 组件路径
	Status          uint8    // 状态
	Redirect        string   // 重定向路径
	IsExternalLinks uint8    // 是否外链
}

// menuEditContext 菜单编辑上下文，保存更新前的状态用于级联判断。
type menuEditContext struct {
	originPids     string // 原始路径
	originPid      uint   // 原始父 ID
	originFullPath string // 原始完整路径
	excludeId      uint   // 排除的当前菜单 ID
}

// applyMenuMutation 执行菜单变更操作（新增/更新）。
// 处理逻辑：
// 1. 验证菜单是否存在（更新时）
// 2. 构建树形层级（pids, level, full_path）
// 3. 检查层级深度
// 4. 填充菜单字段
// 5. 验证唯一字段（code, name, path）
// 6. 验证 API 列表
// 7. 事务保存：菜单数据、级联更新子菜单、更新子菜单数量、同步菜单权限
func (s *MenuService) applyMenuMutation(params *menuMutation) error {
	menu := model.NewMenu()
	editContext := &menuEditContext{
		originPids:     menuRootPid,
		originPid:      0,
		originFullPath: "",
		excludeId:      0,
	}
	// 更新场景：加载现有菜单数据，记录原始状态用于后续级联判断
	if params.Id > 0 {
		if err := menu.GetById(params.Id); err != nil || menu.ID == 0 {
			return e.NewBusinessError(1, "编辑的菜单不存在")
		}
		editContext.originPids = menu.Pids
		editContext.originPid = menu.Pid
		editContext.originFullPath = menu.FullPath
		editContext.excludeId = params.Id
	}
	// 构建树形层级并验证合法性
	if err := s.resolveMenuHierarchy(menu, params); err != nil {
		return err
	}
	// 检查菜单层级深度是否超限
	if menu.Level > maxMenuLevel {
		return e.NewBusinessError(1, "最多只能创建 4 层菜单")
	}

	// 填充菜单字段
	s.assignMenuFields(menu, params)
	// 验证唯一字段
	if err := s.validateUniqueFields(menu, params, editContext.excludeId); err != nil {
		return err
	}
	// 验证 API 列表并去重
	if len(params.ApiList) > 0 {
		apis, err := model.NewApi().FindByIds(params.ApiList)
		if err != nil {
			return err
		}
		params.ApiList = lo.Map(apis, func(api model.Api, _ int) uint {
			return api.ID
		})
	}
	return s.executeEditTransaction(menu, params.ApiList, editContext)
}

// resolveMenuHierarchy 统一处理菜单层级、父级合法性和 full_path 计算，避免主流程在多个小函数间跳转。
// 处理逻辑：
// 1. 判断是否需要更新父级信息（pid 变更或 path 变更）
// 2. 验证父菜单是否存在且不是按钮类型
// 3. 检测环路
// 4. 计算新的 level、pids、full_path
func (s *MenuService) resolveMenuHierarchy(menu *model.Menu, params *menuMutation) error {
	needRefreshByParent := (params.Pid > 0 && params.Pid != menu.Pid) ||
		(params.Pid > 0 && params.Path != menu.Path)
	if !needRefreshByParent {
		// 无需更新父级信息，仅处理顶级菜单场景
		if params.Pid == 0 {
			menu.Level = menuRootLevel
			menu.Pids = menuRootPid
			menu.FullPath = s.buildFullPath(params.Path, rootPath, params.Type)
		}
		menu.Pid = params.Pid
		return nil
	}

	// 验证父菜单是否存在
	parentMenu := model.NewMenu()
	if err := parentMenu.GetById(params.Pid); err != nil || parentMenu.ID == 0 {
		return e.NewBusinessError(1, "上级菜单不存在")
	}
	// 父菜单不能是按钮类型
	if parentMenu.Type == model.BUTTON {
		return e.NewBusinessError(1, "上级菜单不能是按钮类型")
	}
	// 环路检测
	if utils2.WouldCauseCycle(menu.ID, params.Pid, parentMenu.Pids) {
		return e.NewBusinessError(1, "上级菜单不能是当前菜单自身或其子菜单")
	}

	// 计算新的层级和路径
	menu.Level = parentMenu.Level + 1
	menu.Pids = s.buildPids(parentMenu.Pids, parentMenu.ID)
	menu.FullPath = s.buildFullPath(params.Path, parentMenu.FullPath, params.Type)
	menu.Pid = params.Pid
	return nil
}

// buildPids 构建子节点的 pids 路径：父 pids + 父 ID。
func (s *MenuService) buildPids(parentPids string, parentID uint) string {
	return strings.TrimPrefix(fmt.Sprintf("%s,%d", parentPids, parentID), ",")
}

// buildFullPath 构建菜单的完整路径。
// 规则：
// 1. 按钮类型无 full_path
// 2. 已有完整路径前缀（/、http、https）则直接使用
// 3. 否则拼接父路径 + 当前路径
func (s *MenuService) buildFullPath(path, parentPath string, menuType uint8) string {
	if menuType == model.BUTTON {
		return ""
	}
	if parentPath == "" {
		parentPath = rootPath
	}
	if strings.HasPrefix(path, rootPath) ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "http://") {
		return path
	}
	if !strings.HasSuffix(parentPath, "/") {
		parentPath += "/"
	}
	return parentPath + path
}

// assignMenuFields 填充菜单模型字段。
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
	// 按钮类型无 full_path
	if params.Type == model.BUTTON {
		menu.FullPath = ""
	}
}

// validateUniqueFields 验证菜单唯一字段：code、name、full_path。
func (s *MenuService) validateUniqueFields(menu *model.Menu, params *menuMutation, excludeId uint) error {
	// 验证 code 唯一性
	codeExists, err := menu.ExistsExcludeId("code", params.Code, excludeId)
	if err != nil {
		return err
	}
	if params.Code != "" && codeExists {
		return e.NewBusinessError(1, "权限标识已存在")
	}

	// 验证 name 唯一性
	nameExists, err := menu.ExistsExcludeId("name", params.Name, excludeId)
	if err != nil {
		return err
	}
	if params.Name != "" && nameExists {
		return e.NewBusinessError(1, "路由名称已存在")
	}

	// 验证 full_path 唯一性（按钮类型除外）
	if params.Type != model.BUTTON && menu.Path != "" {
		pathExists, err := menu.ExistsExcludeId("full_path", menu.FullPath, excludeId)
		if err != nil {
			return err
		}
		if pathExists {
			return e.NewBusinessError(1, "路由已存在")
		}
	}
	return nil
}

// executeEditTransaction 执行菜单编辑事务。
// 处理逻辑：
// 1. 保存菜单数据
// 2. 级联更新子菜单的 level、pids、full_path
// 3. 更新原父菜单和新父菜单的子菜单数量
// 4. 更新菜单关联的 API 权限
// 5. 同步受影响用户的权限缓存
func (s *MenuService) executeEditTransaction(menu *model.Menu, apiList []uint, editContext *menuEditContext) error {
	db, err := menu.GetDB()
	if err != nil {
		return err
	}

	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		menu.SetDB(tx)
		// 保存菜单数据
		if err := menu.Save(); err != nil {
			return err
		}
		// 级联更新子菜单
		if err := s.updateChildrenLevels(menu, editContext.originPids, editContext.originFullPath, tx); err != nil {
			return err
		}
		// 原父菜单的子菜单数量减 1
		if editContext.originPid > 0 && editContext.originPid != menu.Pid {
			if err := model.UpdateChildrenNum(model.NewMenu(), editContext.originPid, tx); err != nil {
				return err
			}
		}
		// 新父菜单的子菜单数量加 1
		if menu.Pid > 0 && menu.Pid != editContext.originPid {
			if err := model.UpdateChildrenNum(model.NewMenu(), menu.Pid, tx); err != nil {
				return err
			}
		}
		// 更新菜单关联的 API 权限
		return s.updateMenuPermissions(menu, apiList, tx)
	})
	if err != nil {
		return err
	}
	// 同步受影响用户的权限缓存
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByMenus([]uint{menu.ID})
}

// updateChildrenLevels 更新子菜单的层级信息（pids, level, full_path）。
// 当菜单的 pids 或 full_path 变更时，需要级联更新所有子代菜单。
func (s *MenuService) updateChildrenLevels(menu *model.Menu, originPids string, originFullPath string, tx *gorm.DB) error {
	// pids 和 full_path 都未变更，无需级联更新
	if menu.Pids == originPids && menu.FullPath == originFullPath {
		return nil
	}

	// 查询所有子代菜单
	descendantModel := model.NewMenu()
	descendantModel.SetDB(tx)
	descendants, err := descendantModel.FindDescendantsById(menu.ID)
	if err != nil {
		return err
	}
	if len(descendants) == 0 {
		return nil
	}

	// 按 pid 分组，便于递归重建
	childrenByPID := make(map[uint][]*model.Menu, len(descendants))
	for _, child := range descendants {
		childrenByPID[child.Pid] = append(childrenByPID[child.Pid], &child)
	}
	return s.rebuildMenuDescendants(tx, menu, childrenByPID)
}

// rebuildMenuDescendants 递归重建子代菜单的层级信息。
func (s *MenuService) rebuildMenuDescendants(tx *gorm.DB, parent *model.Menu, childrenByPID map[uint][]*model.Menu) error {
	menuModel := model.NewMenu()
	menuModel.SetDB(tx)
	for _, child := range childrenByPID[parent.ID] {
		// 重建子菜单的 pids、level、full_path
		child.Pids = s.buildPids(parent.Pids, parent.ID)
		child.Level = parent.Level + 1
		child.FullPath = s.buildFullPath(child.Path, parent.FullPath, child.Type)
		if child.Type == model.BUTTON {
			child.FullPath = ""
		}
		// 批量更新数据库
		if err := menuModel.UpdateById(child.ID, map[string]any{
			"pids":      child.Pids,
			"level":     child.Level,
			"full_path": child.FullPath,
		}); err != nil {
			return err
		}
		// 递归处理下一级子菜单
		if err := s.rebuildMenuDescendants(tx, child, childrenByPID); err != nil {
			return err
		}
	}
	return nil
}

// updateMenuPermissions 更新菜单关联的 API 权限。
// 使用差分算法：计算需要删除和新增的 API ID，只变更差异部分。
func (s *MenuService) updateMenuPermissions(menu *model.Menu, apiList []uint, tx ...*gorm.DB) error {
	menuApiMap := model.NewMenuApiMap()
	if len(tx) > 0 {
		menuApiMap.SetDB(tx[0])
	}

	// 查询菜单当前已关联的 API ID 列表
	existingMaps, err := model.ListE(menuApiMap, "menu_id = ?", []any{menu.ID}, model.ListOptionalParams{
		SelectFields: []string{"api_id"},
	})
	if err != nil {
		return err
	}

	existingIDs := lo.Map(existingMaps, func(m *model.MenuApiMap, _ int) uint {
		return m.ApiId
	})
	apiList = lo.Uniq(apiList)
	// 计算差异
	toDelete, toAdd := lo.Difference(existingIDs, apiList)

	// 删除差异 API 关联
	if len(toDelete) > 0 {
		if err := menuApiMap.DeleteWhere("menu_id = ? AND api_id IN (?)", []any{menu.ID, toDelete}...); err != nil {
			return err
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	// 新增 API 关联
	newMappings := lo.Map(toAdd, func(apiID uint, _ int) *model.MenuApiMap {
		return &model.MenuApiMap{MenuId: menu.ID, ApiId: apiID}
	})
	return menuApiMap.CreateBatch(newMappings)
}

// UpdateAllMenuPermissions 批量更新所有菜单的权限到 Casbin。
func (s *MenuService) UpdateAllMenuPermissions() error {
	return access.NewPermissionSyncCoordinator().SyncAll()
}
